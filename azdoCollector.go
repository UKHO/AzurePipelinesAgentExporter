package main

import (
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"./azdo"
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

	runningJobsDesc = prometheus.NewDesc(
		"tfs_pool_running_jobs",
		"Total of running jobs for pool",
		[]string{"pool"},
		nil,
	)
)

type azDoCollector struct {
	AzDoClient        *azdo.AzDoClient
	ignoreHostedPools bool
}

func newAzDoCollector(az azdo.AzDoClient, ignoreHostedPools bool) *azDoCollector {
	return &azDoCollector{AzDoClient: &az, ignoreHostedPools: ignoreHostedPools}
}

func (azc azDoCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(azc, ch)
}

type result struct {
	pool        azdo.Pool
	agents      []azdo.Agent
	currentJobs []azdo.Job
}

type agentMetric struct {
	count   float64
	enabled bool
	status  string
	pool    string
}

func (azc azDoCollector) Collect(ch chan<- prometheus.Metric) {

	start := time.Now()

	//Get all the pools from AzDo
	pools, err := azc.AzDoClient.Pools(azc.ignoreHostedPools)
	if err != nil {
		log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "error": err}).Error(" Scrape Failed. Could not retrive pools.")
		return
	}
	log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "poolCount": len(pools)}).Debug("Retrieved pools")

	//Get all agents from all pools,
	chanAgents, errOccurred := azc.collectAgents(pools)
	chanCurrentJobs := azc.collectCurrentJobs(chanAgents)
	chanCalculatedMetrics := azc.calculateMetrics(chanCurrentJobs)
	chanBufferedMetrics := azc.bufferMetrics(chanCalculatedMetrics, errOccurred) //Does not start writing to the out chan until the in chan is closed. ErrOccured must be false to write anything to out chan

	for metric := range chanBufferedMetrics {
		ch <- metric
	}

	log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name}).Info("Scraped agents")

	// Send time it has take to run this scrape
	ch <- prometheus.MustNewConstMetric(
		installedBuildAgentsDurationDesc,
		prometheus.GaugeValue,
		time.Since(start).Seconds(),
	)
}

func (azc *azDoCollector) collectAgents(pools []azdo.Pool) (<-chan result, bool) {
	errOccurred := false
	out := make(chan result)
	var wg sync.WaitGroup

	// For each pool, spin up a go routine, retrive the agents for that pool and pass both the pool and agents along into the channel for the next step
	for _, p := range pools {
		wg.Add(1)
		go func(p azdo.Pool) {
			agents, err := azc.AzDoClient.Agents(p.ID)
			if err != nil {
				errOccurred = true
				log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "poolId": p.ID, "err": err}).Error("Failed to retrieve agents for pool")
			}
			log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "poolId": p.ID, "agentsInPoolCount": len(agents)}).Debug("Retrieved agents for pool")
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

func (azc *azDoCollector) collectCurrentJobs(in <-chan result) <-chan result {
	out := make(chan result)

	go func() {
		for result := range in {
			currentJobs, err := azc.AzDoClient.CurrentJobs(result.pool.ID)
			if err != nil {
				log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "poolId": result.pool.ID, "err": err}).Error("Failed to retrieve queued jobs for pool")
			}
			log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "poolId": result.pool.ID, "currentJobsInPoolCount": len(currentJobs)}).Debug("Retrieved current jobs for pools")
			result.currentJobs = currentJobs

			out <- result
		}
		close(out)
	}()

	return out
}

func (azc *azDoCollector) calculateMetrics(in <-chan result) <-chan prometheus.Metric {
	out := make(chan prometheus.Metric)

	go func() {
		for result := range in {
			for _, v := range calculateAgentMetrics(result) {
				out <- v
			}
			out <- calculateQueuedJobMetrics(result)
			out <- calculateRunningJobMetrics(result)
		}
		close(out)

	}()
	return out
}

func (azc *azDoCollector) bufferMetrics(in <-chan prometheus.Metric, errOccurred bool) <-chan prometheus.Metric {
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
			log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name}).Error("Metrics not being exposed due to previous error")
			close(out)
			return
		}

		log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name}).Info("No errors detected collecting metric. Exposing metrics")
		for _, m := range metrics {
			out <- m
		}

		close(out)

	}()
	return out
}

func calculateRunningJobMetrics(result result) prometheus.Metric {
	count := 0
	for _, j := range result.currentJobs {
		if !j.AssignTime.IsZero() { //Then the job has started and is therefore queued
			count++
		}
	}

	return prometheus.MustNewConstMetric(
		runningJobsDesc,
		prometheus.GaugeValue,
		float64(count),
		result.pool.Name,
	)
}

func calculateQueuedJobMetrics(result result) prometheus.Metric {

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

func calculateAgentMetrics(result result) []prometheus.Metric {
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
	return promMetrics
}
