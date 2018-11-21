package main

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	installedBuildAgentsDesc = prometheus.NewDesc(
		"tfs_build_agents_total",
		"Total of installed build agents",
		[]string{"enabled", "status", "pool"},
		nil,
	)

	installedBuildAgentsDurationDesc = prometheus.NewDesc(
		"tfs_build_agents_total_scrape_duration_seconds",
		"Duration of time it took to scrape total of installed build agents",
		[]string{},
		nil,
	)
)

type tfsCollector struct {
	tfs               *tfs
	ignoreHostedPools bool
}

func newTFSCollector(t tfs, ignoreHostedPools bool) *tfsCollector {
	return &tfsCollector{tfs: &t, ignoreHostedPools: ignoreHostedPools}
}

func (tc tfsCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(tc, ch)
}

type result struct {
	pool   pool
	agents []agent
}

type agentMetric struct {
	count   float64
	enabled bool
	status  string
	pool    string
}

func (tc tfsCollector) Collect(ch chan<- prometheus.Metric) {

	start := time.Now()

	//Get all the pools from TFS
	pools, err := tc.tfs.pools(tc.ignoreHostedPools)
	if err != nil {
		log.WithFields(log.Fields{"serverName": tc.tfs.Name, "error": err}).Error(" Scrape Failed. Could not retrive pools.")
		return
	}
	log.WithFields(log.Fields{"serverName": tc.tfs.Name, "poolCount": len(pools)}).Debug("Retrieved pools")

	//Get all agents from all pools,
	chanAgents := tc.collectAgents(pools)
	chanRawMetrics := tc.calculateMetrics(chanAgents)
	chanFormattedMetrics := tc.formatMetrics(chanRawMetrics)

	for metric := range chanFormattedMetrics {
		ch <- metric
	}

	// Send time it has take to run this scrape
	ch <- prometheus.MustNewConstMetric(
		installedBuildAgentsDurationDesc,
		prometheus.GaugeValue,
		time.Since(start).Seconds(),
	)
}

// if err != nil {
// 	log.WithFields(log.Fields{"serverName": tc.tfs.Name, "error": err}).Error("Failed to retrieve agents")
// 	return
// }
// log.WithFields(log.Fields{"serverName": tc.tfs.Name, "totalAgents": len(agents)}).Info("Scraped agents")

func (tc *tfsCollector) collectAgents(pools []pool) <-chan result {
	out := make(chan result)

	// For each pool, spin up a go routine, retrive the agents for that pool and pass both the pool and agents along into the channel for the next step
	for _, p := range pools {
		go func(p pool) {
			agents, err := tc.tfs.agents(p.ID)
			if err != nil {

			}
			log.WithFields(log.Fields{"serverName": tc.tfs.Name, "poolId": p.ID, "agentsInPoolCount": len(agents)}).Debug("Retrieved agents for pool")
			out <- result{pool: p, agents: agents}
		}(p)
	}

	return out
}

func (tc *tfsCollector) calculateMetrics(in <-chan result) <-chan []agentMetric {
	out := make(chan []agentMetric)

	go func() {
		for n := range in {

			m := make(map[string]agentMetric)

			for _, a := range n.agents {
				var key = strconv.FormatBool(a.Enabled) + a.Status // looks like "trueOnline"

				// Does the key exist in the map?
				// If it does increase the count on the value else create a new value
				// assign the value back to the map
				v, ok := m[key]
				if ok {
					v.count++
				} else {
					v = agentMetric{count: 1, enabled: a.Enabled, status: a.Status, pool: n.pool.Name}
				}

				m[key] = v
			}

			//Take all the values out the map and put them into a slice, because it is prettier
			values := []agentMetric{}
			for _, value := range m {
				values = append(values, value)
			}

			out <- values
		}
		close(out)
	}()

	return out
}

func (tc *tfsCollector) formatMetrics(in <-chan []agentMetric) <-chan prometheus.Metric {

	out := make(chan prometheus.Metric)

	go func() {
		for n := range in {
			for _, kv := range n {
				out <- prometheus.MustNewConstMetric(
					installedBuildAgentsDesc,
					prometheus.GaugeValue,
					kv.count,
					strconv.FormatBool(kv.enabled),
					kv.status,
					kv.pool,
				)
			}
		}
		close(out)
	}()

	return out
}
