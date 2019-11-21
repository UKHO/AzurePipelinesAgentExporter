package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff"

	log "github.com/sirupsen/logrus"
)

type tfs struct {
	client            *http.Client
	Name              string
	Address           string
	DefaultCollection string
	AccessToken       string
}

func (t *tfs) agents(poolID int) ([]agent, error) {

	//Build request
	var url = t.buildURL("/_apis/distributedtask/pools/" + strconv.Itoa(poolID) + "/agents?includeCapabilities=false&includeAssignedRequest=true")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []agent{}, fmt.Errorf("Could not generate request to find all agents in poolID %v - %v", poolID, err)
	}
	req.SetBasicAuth("", t.AccessToken)

	//Make request
	responseData, err := t.makeRequest(req)
	if err != nil {
		return []agent{}, err
	}

	//	Turn response into JSON
	are := agentResponseEnvelope{}
	err = json.Unmarshal(responseData, &are)
	if err != nil {
		return []agent{}, fmt.Errorf("Failed to convert to JSON - %v", err)
	}

	// Need to add the pool name to the agent so can be a label on metric
	// Very ugly. Can do better.
	for _, a := range are.Agents {
		a.poolID = poolID
	}

	return are.Agents, nil
}

// It would be nice to query TFS directly for non-hosted agents. Ideally via a query string on the API but not possible- "pools?ishosted=false"
func (t *tfs) pools(ignoreHosted bool) ([]pool, error) {

	//Build request
	var url = t.buildURL("/_apis/distributedtask/pools")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []pool{}, fmt.Errorf("Could not generate request to find all agent pools %v", err)
	}
	req.SetBasicAuth("", t.AccessToken)

	//Make request
	responseData, err := t.makeRequest(req)
	if err != nil {
		return []pool{}, err
	}

	//	Turn response into JSON
	pre := poolResponseEnvelope{}
	err = json.Unmarshal(responseData, &pre)
	if err != nil {
		return []pool{}, fmt.Errorf("Failed to convert to JSON - %v", err)
	}

	// Remove hosted pools
	if ignoreHosted {
		var nonHostedPools []pool
		for _, p := range pre.Pools {
			if p.IsHosted == false {
				nonHostedPools = append(nonHostedPools, p)
			}
		}
		pre.Pools = nonHostedPools
	}

	return pre.Pools, nil
}

func (t *tfs) currentJobs(poolID int) ([]job, error) {
	//Build request
	var url = t.buildURL("/_apis/distributedtask/pools/" + strconv.Itoa(poolID) + "/jobrequests/?completedRequestCount=0")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []job{}, fmt.Errorf("Could not generate request to find all queued jobs %v", err)
	}
	req.SetBasicAuth("", t.AccessToken)

	//Make request
	responseData, err := t.makeRequest(req)
	if err != nil {
		return []job{}, err
	}

	//	Turn response into type from JSON
	jre := jobResponseEnvelope{}
	err = json.Unmarshal(responseData, &jre)
	if err != nil {
		return []job{}, fmt.Errorf("Failed to convert to JSON - %v", err)
	}

	return jre.Jobs, nil
}

func (t *tfs) makeRequest(req *http.Request) ([]byte, error) {

	var (
		responseData []byte
		err          error
	)

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 30 * time.Second

	notify := func(err error, ti time.Duration) {
		log.WithFields(log.Fields{"serverName": t.Name, "URL": req.URL, "error": err}).Warning("Retrying HTTP request")
	}

	retry := func() error {
		responseData, err = t.makeHTTPRequest(req)
		return err
	}

	e := backoff.RetryNotify(retry, b, notify)
	if e != nil {
		return []byte{}, e
	}

	return responseData, nil
}

func (t *tfs) makeHTTPRequest(req *http.Request) ([]byte, error) {

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return []byte{}, fmt.Errorf("Call to %v failed: %v", req.URL, err)
	}
	defer resp.Body.Close()
	log.WithFields(log.Fields{"serverName": t.Name, "URL": req.URL, "StatusCode": resp.StatusCode}).Trace("Made HTTP request")

	// Read body of response
	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, fmt.Errorf("Failed to read body %v", err)
	}

	return responseData, nil
}

func (t *tfs) buildURL(url string) string {
	var baseURL string
	if t.DefaultCollection != "" {
		baseURL = t.Address + "/" + t.DefaultCollection
	} else {
		baseURL = t.Address
	}

	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}

	return baseURL + url
}
