package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

type tfs struct {
	client            *http.Client
	Name              string
	Address           string
	DefaultCollection string
	AccessToken       string
}

// It would be nice to query TFS directly for non-hosted agents. Ideally via a query string on the API but not possible- "pools?ishosted=false"

func (t tfs) GetAllAgents(ignoreHostedAgents bool) ([]agent, error) {

	//Get all the pools from TFS
	pools, err := t.pools()
	if err != nil {
		return []agent{}, err
	}
	log.WithFields(log.Fields{"serverName": t.Name, "poolCount": len(pools)}).Debug("Retrieved pools")

	// Strip out any hosted pools
	if ignoreHostedAgents {
		var nonHostedPools []pool
		for _, p := range pools {
			if p.IsHosted == false {
				nonHostedPools = append(nonHostedPools, p)
			}
		}
		pools = nonHostedPools
	}

	ach, errc := make(chan []agent), make(chan error)
	defer func() {
		close(ach)
		close(errc)
	}()

	allAgents := []agent{}

	//TODO: Do something decent with errors. It needs to kill all
	//Get all agents in each pool and collate them
	for _, pool := range pools {

		go func(id int) {
			agentsInPool, err := t.agents(id)
			if err != nil {
				errc <- err
				return
			}
			log.WithFields(log.Fields{"serverName": t.Name, "poolId": id, "agentsInPoolCount": len(agentsInPool)}).Debug("Retrieved agents for pool")

			ach <- agentsInPool
		}(pool.ID)
	}

	for i := 0; i < len(pools); i++ {
		select {
		case agentsInPool := <-ach:
			allAgents = append(allAgents, agentsInPool...)
		case err := <-errc:
			//return []agent{}, err
			log.WithFields(log.Fields{"serverName": t.Name, "error": err}).Error("Failed to get agents for a pool")
		}
	}

	// This might be useless
	log.WithFields(log.Fields{"serverName": t.Name, "allAgentsCount": len(allAgents)}).Debug("All agents retrieved")

	return allAgents, nil
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

func (t *tfs) pools() ([]pool, error) {

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

	return pre.Pools, nil
}

func (t *tfs) makeRequest(req *http.Request) ([]byte, error) {

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
