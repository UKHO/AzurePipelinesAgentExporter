package main

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"./azdo"
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

func (azc azDoCollector) Collect(publishMetrics chan<- prometheus.Metric) {

	start := time.Now()

	//Get all the pools from AzDo
	pools, err := azc.AzDoClient.Pools(azc.ignoreHostedPools)
	if err != nil {
		log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "error": err}).Error(" Scrape Failed. Could not retrive pools.")
		return
	}
	log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "poolCount": len(pools)}).Debug("Retrieved pools")

	// Pipeline for scraping and calculating metrics.
	// Each returns a channel which the next step consumes.
	// scrapeAgents returns a channel of metricContexts which contains the agents for a pool.
	// scrapeCurrentJobs then consumes this channel and augments the metricContexts with information about the currentJobs

	chanAgents, errOccurred := azc.scrapeAgents(pools)
	chanCurrentJobs := azc.scrapeCurrentJobs(chanAgents)
	chanCalculatedMetrics := azc.calculateMetrics(chanCurrentJobs)
	chanBufferedMetrics := azc.bufferMetrics(chanCalculatedMetrics, errOccurred) //Buffers and blocks until the in chan is closed. ErrOccured must be false to write anything to out chan

	// Publish the buffered metrics
	for metric := range chanBufferedMetrics {
		publishMetrics <- metric
	}

	log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name}).Info("Scraped agents")

	// Time it has take to run this scrape
	publishMetrics <- prometheus.MustNewConstMetric(
		installedBuildAgentsDurationDesc,
		prometheus.GaugeValue,
		time.Since(start).Seconds(),
	)
}

func (azc *azDoCollector) scrapeAgents(pools []azdo.Pool) (<-chan metricsContext, bool) {
	errOccurred := false
	metricsContextChanOut := make(chan metricsContext) //Channel to pass metricsContext along to for next part of the pipeline
	var wg sync.WaitGroup

	// For each pool, spin up a go routine, retrive the agents for that pool and pass both the pool and agents along into the channel for the next step
	for _, pool := range pools {
		wg.Add(1)
		go func(p azdo.Pool) {
			agents, err := azc.AzDoClient.Agents(p.ID) //Get all Agents for pool
			if err != nil {
				errOccurred = true
				log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "poolId": p.ID, "err": err}).Error("Failed to retrieve agents for pool")
			}
			log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "poolId": p.ID, "agentsInPoolCount": len(agents)}).Debug("Retrieved agents for pool")
			metricsContextChanOut <- metricsContext{pool: p, agents: agents}
			wg.Done()
		}(pool)
	}

	go func() {
		wg.Wait()
		close(metricsContextChanOut)
	}()

	return metricsContextChanOut, errOccurred
}

func (azc *azDoCollector) scrapeCurrentJobs(metricsContextChanIn <-chan metricsContext) <-chan metricsContext {
	metricsContextChanOut := make(chan metricsContext)

	go func() {
		for metricsContext := range metricsContextChanIn {
			currentJobs, err := azc.AzDoClient.CurrentJobs(metricsContext.pool.ID)
			if err != nil {
				log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "poolId": metricsContext.pool.ID, "err": err}).Error("Failed to retrieve queued jobs for pool")
			}
			log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name, "poolId": metricsContext.pool.ID, "currentJobsInPoolCount": len(currentJobs)}).Debug("Retrieved current jobs for pools")

			metricsContext.currentJobs = currentJobs // Augment the metrics context with the current jobs for this pool

			metricsContextChanOut <- metricsContext
		}
		close(metricsContextChanOut)
	}()

	return metricsContextChanOut
}

func (azc *azDoCollector) calculateMetrics(metricsContextChanIn <-chan metricsContext) <-chan prometheus.Metric {
	metrics := make(chan prometheus.Metric)

	go func() {
		for metricsContext := range metricsContextChanIn {

			agentMetrics := calculateAgentMetrics(metricsContext)
			for _, agentMetric := range agentMetrics {
				metrics <- agentMetric
			}

			jobMetrics := calculateJobMetrics(metricsContext)
			for _, jobMetric := range jobMetrics {
				metrics <- jobMetric
			}

		}
		close(metrics)

	}()
	return metrics
}

func (azc *azDoCollector) bufferMetrics(metricsIn <-chan prometheus.Metric, errOccurred bool) <-chan prometheus.Metric {
	metricsOut := make(chan prometheus.Metric)

	go func() {
		bufferedMetrics := []prometheus.Metric{}

		// Will not exit range until 'metricsIn' chan is closed earlier in the pipeline thus blocking exposing the metrics
		// If there are any errors then gives chance for them not to published
		for metric := range metricsIn {
			bufferedMetrics = append(bufferedMetrics, metric)
		}

		if errOccurred {
			log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name}).Error("Metrics not exposed due to previous error")
			close(metricsOut)
			return
		}

		log.WithFields(log.Fields{"serverName": azc.AzDoClient.Name}).Info("No errors detected collecting metric. Exposing metrics")
		for _, metric := range bufferedMetrics {
			metricsOut <- metric
		}

		close(metricsOut)

	}()
	return metricsOut
}

// Contains all the information needed to calculate the metrics
type metricsContext struct {
	pool        azdo.Pool
	agents      []azdo.Agent
	currentJobs []azdo.Job
}
