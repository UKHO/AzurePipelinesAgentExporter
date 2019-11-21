package main

import (
	"strconv"
	"sync"
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

	queuedJobsDesc = prometheus.NewDesc(
		"tfs_pool_queued_jobs",
		"Total of queued jobs for pool",
		[]string{"pool"},
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
	pool        pool
	agents      []agent
	currentJobs []job
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
	chanAgents, errOccurred := tc.collectAgents(pools)
	chanCurrentJobs := tc.collectCurrentJobs(chanAgents)
	chanRawMetrics := tc.calculateMetrics(chanCurrentJobs)
	chanFormattedMetrics := tc.formatMetrics(chanRawMetrics)
	chanBufferedMetrics := tc.bufferMetrics(chanFormattedMetrics, errOccurred) //Does not start writing to the out chan until the in chan is closed. ErrOccured must be false to write anything to out chan

	for metric := range chanBufferedMetrics {
		ch <- metric
	}

	log.WithFields(log.Fields{"serverName": tc.tfs.Name}).Info("Scraped agents")

	// Send time it has take to run this scrape
	ch <- prometheus.MustNewConstMetric(
		installedBuildAgentsDurationDesc,
		prometheus.GaugeValue,
		time.Since(start).Seconds(),
	)
}

func (tc *tfsCollector) collectAgents(pools []pool) (<-chan result, bool) {
	errOccurred := false
	out := make(chan result)
	var wg sync.WaitGroup

	// For each pool, spin up a go routine, retrive the agents for that pool and pass both the pool and agents along into the channel for the next step
	for _, p := range pools {
		wg.Add(1)
		go func(p pool) {
			agents, err := tc.tfs.agents(p.ID)
			if err != nil {
				errOccurred = true
				log.WithFields(log.Fields{"serverName": tc.tfs.Name, "poolId": p.ID, "err": err}).Error("Failed to retrieve agents for pool")
			}
			log.WithFields(log.Fields{"serverName": tc.tfs.Name, "poolId": p.ID, "agentsInPoolCount": len(agents)}).Debug("Retrieved agents for pool")
			out <- result{pool: p, agents: agents}
			wg.Done()
		}(p)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out, errOccurred
}

func (tc *tfsCollector) collectCurrentJobs(in <-chan result) <-chan result {
	out := make(chan result)

	go func() {
		for result := range in {
			currentJobs, err := tc.tfs.currentJobs(result.pool.ID)
			if err != nil {
				log.WithFields(log.Fields{"serverName": tc.tfs.Name, "poolId": result.pool.ID, "err": err}).Error("Failed to retrieve queued jobs for pool")
			}
			log.WithFields(log.Fields{"serverName": tc.tfs.Name, "poolId": result.pool.ID, "currentJobsInPoolCount": len(currentJobs)}).Debug("Retrieved current jobs for pools")
			result.currentJobs = currentJobs

			out <- result
		}
		close(out)
	}()

	return out
}

func (tc *tfsCollector) calculateMetrics(in <-chan result) <-chan []agentMetric {
	out := make(chan []agentMetric)
	//out1 := make(chan prometheus.Metric)

	go func() {
		for result := range in {
			out <- calculateAgentMetrics(result)
			//		out1 <- calculateQueuedJobMetrics(result)
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

func (tc *tfsCollector) bufferMetrics(in <-chan prometheus.Metric, errOccurred bool) <-chan prometheus.Metric {
	out := make(chan prometheus.Metric)

	go func() {
		metrics := []prometheus.Metric{}

		// Will not exit range until 'in' chan is closed
		// Blocks exposing the metrics until all metrics have been calculated as 'in' will only close when all metrics have been calculated
		// If there are any errors then gives chance not to publish them
		for m := range in {
			metrics = append(metrics, m)
		}

		if errOccurred {
			log.WithFields(log.Fields{"serverName": tc.tfs.Name}).Error("Metrics not being exposed due to previous error")
			close(out)
			return
		}

		log.WithFields(log.Fields{"serverName": tc.tfs.Name}).Info("No errors detected collecting metric. Exposing metrics")
		for _, m := range metrics {
			out <- m
		}

		close(out)

	}()
	return out
}

func calculateQueuedJobMetrics(result result) prometheus.Metric {
	//discover the current queued jobs
	count := 0
	for _, j := range result.currentJobs {
		if j.AssignTime.IsZero() { //Then the job hasn't started and is therefore queued
			count++
		}
	}

	return prometheus.MustNewConstMetric(
		queuedJobsDesc,
		prometheus.GaugeValue,
		float64(count),
		result.pool.Name,
	)
}

func calculateAgentMetrics(result result) []agentMetric {
	m := make(map[string]agentMetric)

	for _, agent := range result.agents {
		var state = strconv.FormatBool(agent.Enabled) + agent.Status // looks like "trueOnline"

		// Does the state already exist in the map?
		// If it does increase the count on the value else create a new value
		// assign the value back to the map
		v, ok := m[state]
		if ok {
			v.count++
		} else {
			v = agentMetric{count: 1, enabled: agent.Enabled, status: agent.Status, pool: result.pool.Name}
		}

		m[state] = v
	}

	promMetrics := []prometheus.Metric{}
	for _, p := range m {

		promMetric := prometheus.MustNewConstMetric(
			installedBuildAgentsDesc,
			prometheus.GaugeValue,
			p.count,
			strconv.FormatBool(p.enabled),
			p.status,
			p.pool)

		promMetrics = append(promMetrics, promMetric)
	}

	// Take all the values out the map and put them into a slice, because it is prettier
	values := []agentMetric{}
	for _, value := range m {
		values = append(values, value)
	}

	// for _, kv := range values {
	// 	out <- prometheus.MustNewConstMetric(
	// 		installedBuildAgentsDesc,
	// 		prometheus.GaugeValue,
	// 		kv.count,
	// 		strconv.FormatBool(kv.enabled),
	// 		kv.status,
	// 		kv.pool,
	// 	)
	// }

	return values

}
