package azdo

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

type AzDoClient struct {
	Client            *http.Client
	Name              string
	Address           string
	DefaultCollection string
	AccessToken       string
}

func (az *AzDoClient) Agents(poolID int) ([]Agent, error) {

	// Build request
	var url = az.buildURL("/_apis/distributedtask/pools/" + strconv.Itoa(poolID) + "/agents?includeCapabilities=false&includeAssignedRequest=true")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []Agent{}, fmt.Errorf("Could not generate request to find all agents in poolID %v - %v", poolID, err)
	}
	req.SetBasicAuth("", az.AccessToken)

	// Make request
	responseData, err := az.makeRequest(req)
	if err != nil {
		return []Agent{}, err
	}

	// Turn response into JSON
	are := agentResponseEnvelope{}
	err = json.Unmarshal(responseData, &are)
	if err != nil {
		return []Agent{}, fmt.Errorf("Failed to convert to JSON - %v", err)
	}

	// Need to add the pool name to the agent so can be a label on metric
	// Very ugly. Can do better.
	for _, a := range are.Agents {
		a.poolID = poolID
	}

	return are.Agents, nil
}

// It would be nice to query TFS directly for non-hosted agents. Ideally via a query string on the API but not possible- "pools?ishosted=false"
func (az *AzDoClient) Pools(ignoreHosted bool) ([]Pool, error) {

	//Build request
	var url = az.buildURL("/_apis/distributedtask/pools")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []Pool{}, fmt.Errorf("Could not generate request to find all agent pools %v", err)
	}
	req.SetBasicAuth("", az.AccessToken)

	//Make request
	responseData, err := az.makeRequest(req)
	if err != nil {
		return []Pool{}, err
	}

	//	Turn response into JSON
	pre := poolResponseEnvelope{}
	err = json.Unmarshal(responseData, &pre)
	if err != nil {
		return []Pool{}, fmt.Errorf("Failed to convert to JSON - %v", err)
	}

	// Remove hosted pools
	if ignoreHosted {
		var nonHostedPools []Pool
		for _, p := range pre.Pools {
			if p.IsHosted == false {
				nonHostedPools = append(nonHostedPools, p)
			}
		}
		pre.Pools = nonHostedPools
	}

	return pre.Pools, nil
}

func (az *AzDoClient) CurrentJobs(poolID int) ([]Job, error) {
	// Build request
	var url = az.buildURL("/_apis/distributedtask/pools/" + strconv.Itoa(poolID) + "/jobrequests/?completedRequestCount=0")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []Job{}, fmt.Errorf("Could not generate request to find all queued jobs %v", err)
	}
	req.SetBasicAuth("", az.AccessToken)

	// Make request
	responseData, err := az.makeRequest(req)
	if err != nil {
		return []Job{}, err
	}

	// Turn response into type from JSON
	jre := jobResponseEnvelope{}
	err = json.Unmarshal(responseData, &jre)
	if err != nil {
		return []Job{}, fmt.Errorf("Failed to convert to JSON - %v", err)
	}

	return jre.Jobs, nil
}

func (az *AzDoClient) makeRequest(req *http.Request) ([]byte, error) {

	var (
		responseData []byte
		err          error
	)

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 30 * time.Second

	notify := func(err error, ti time.Duration) {
		log.WithFields(log.Fields{"serverName": az.Name, "URL": req.URL, "error": err}).Warning("Retrying HTTP request")
	}

	retry := func() error {
		responseData, err = az.makeHTTPRequest(req)
		return err
	}

	e := backoff.RetryNotify(retry, b, notify)
	if e != nil {
		return []byte{}, e
	}

	return responseData, nil
}

func (az *AzDoClient) makeHTTPRequest(req *http.Request) ([]byte, error) {

	// Send request
	resp, err := az.Client.Do(req)
	if err != nil {
		return []byte{}, fmt.Errorf("Call to %v failed: %v", req.URL, err)
	}
	defer resp.Body.Close()
	log.WithFields(log.Fields{"serverName": az.Name, "URL": req.URL, "StatusCode": resp.StatusCode}).Trace("Made HTTP request")

	// Read body of response
	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, fmt.Errorf("Failed to read body %v", err)
	}

	return responseData, nil
}

func (az *AzDoClient) buildURL(url string) string {
	var baseURL string
	if az.DefaultCollection != "" {
		baseURL = az.Address + "/" + az.DefaultCollection
	} else {
		baseURL = az.Address
	}

	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}

	return baseURL + url
}
