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
		[]string{"enabled", "status"},
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

func (tc tfsCollector) Collect(ch chan<- prometheus.Metric) {

	start := time.Now()

	agents, err := tc.tfs.GetAllAgents(tc.ignoreHostedPools)
	if err != nil {
		log.WithFields(log.Fields{"serverName": tc.tfs.Name, "error": err}).Error("Failed to retrieve agents")
		return
	}
	log.WithFields(log.Fields{"serverName": tc.tfs.Name, "totalAgents": len(agents)}).Info("Scraped agents")

	// Transform list of all agents into the metrics with individual labels
	// Iterate over them and add them to new a map and increment the count for each possible combo of labels
	type agentMetric struct {
		count   float64
		enabled bool
		status  string
	}

	m := make(map[string]agentMetric)

	for _, a := range agents {
		var key = strconv.FormatBool(a.Enabled) + a.Status // looks like "trueOnline"

		// Does the key exist in the map?
		// If it does increase the count on the value else create a new value
		// assign the value back to the map
		v, ok := m[key]
		if ok {
			v.count++
		} else {
			v = agentMetric{count: 1, enabled: a.Enabled, status: a.Status}
		}

		m[key] = v
	}

	//Iterate over the map and foreach entry, create a metric for it with some labels
	for _, kv := range m {
		ch <- prometheus.MustNewConstMetric(
			installedBuildAgentsDesc,
			prometheus.GaugeValue,
			kv.count,
			strconv.FormatBool(kv.enabled),
			kv.status,
		)
	}

	// Send time it has take to run this scrape
	ch <- prometheus.MustNewConstMetric(
		installedBuildAgentsDurationDesc,
		prometheus.GaugeValue,
		time.Since(start).Seconds(),
	)

}
