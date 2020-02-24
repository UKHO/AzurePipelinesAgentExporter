package main

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
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

	totalJobsDesc = prometheus.NewDesc(
		"tfs_pool_total_jobs",
		"Total of jobs for pool",
		[]string{"pool"},
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

func calculateHistograms(metricContext metricsContext) []prometheus.Metric {

	totalTimes := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "tfs_pool_job_total_length_secs",
		Help:        "Total length of job duration for pool",
		Buckets:     calculateBuckets(),
		ConstLabels: map[string]string{"pool": metricContext.pool.Name},
	})

	for _, job := range metricContext.finishedJobs {
		totalTime := job.FinishTime.Sub(job.QueueTime)
		totalTimes.Observe(totalTime.Seconds())
	}

	queueTimes := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "tfs_pool_job_queue_length_secs",
		Help:        "Total length of queue duration for pool",
		Buckets:     prometheus.ExponentialBuckets(1, 2, 10), // 10 buckets, starting at one, doubling
		ConstLabels: map[string]string{"pool": metricContext.pool.Name},
	})

	for _, job := range metricContext.finishedJobs {
		queueTime := job.ReceiveTime.Sub(job.QueueTime) // Time received by the agent - Time queued by the user
		queueTimes.Observe(queueTime.Seconds())
	}

	jobTimes := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "tfs_pool_job_running_length_secs",
		Help:        "Total length of queue duration for pool",
		Buckets:     calculateBuckets(),
		ConstLabels: map[string]string{"pool": metricContext.pool.Name},
	})

	for _, job := range metricContext.finishedJobs {
		jobTime := job.FinishTime.Sub(job.ReceiveTime)
		jobTimes.Observe(jobTime.Seconds())
	}
	return []prometheus.Metric{
		totalTimes,
		queueTimes,
		jobTimes,
	}
}

func calculateBuckets() []float64 {
	var b = buckets(0, 15, 8)                       // start at 0, gap of 15 between buckets and 10 of them
	b = append(b, buckets(b[len(b)-1], 30, 10)...)  // start of the last value of previous slice, gap of 30 between buckets and 10 of them
	b = append(b, buckets(b[len(b)-1], 60, 28)...)  // start of the last value of previous slice, gap of 30 between buckets and 10 of them
	b = append(b, buckets(b[len(b)-1], 300, 11)...) // start of the last value of previous slice, gap of 300 between buckets and 11 of them
	return b
}

func buckets(start float64, gap float64, count int) []float64 {
	var s []float64
	var currentBucket = start

	for i := 0; i < count; i++ {
		currentBucket = currentBucket + gap
		s = append(s, currentBucket)
	}

	return s
}

func calculateJobMetrics(metricContext metricsContext) []prometheus.Metric {

	queuedTotal := 0
	runningTotal := 0

	for _, currentJob := range metricContext.currentJobs {
		if currentJob.AssignTime.IsZero() { //Then the job hasn't started and is therefore queued
			queuedTotal++
		} else {
			runningTotal++
		}
	}

	calculatedMetrics := []prometheus.Metric{
		prometheus.MustNewConstMetric(
			totalJobsDesc,
			prometheus.GaugeValue,
			float64(len(metricContext.currentJobs)),
			metricContext.pool.Name,
		),
		prometheus.MustNewConstMetric(
			runningJobsDesc,
			prometheus.GaugeValue,
			float64(runningTotal),
			metricContext.pool.Name,
		),
		prometheus.MustNewConstMetric(
			queuedJobsDesc,
			prometheus.GaugeValue,
			float64(queuedTotal),
			metricContext.pool.Name,
		),
	}

	calculatedMetrics = append(calculatedMetrics, calculateHistograms(metricContext)...)

	return calculatedMetrics

}

func calculateAgentMetrics(metricContext metricsContext) []prometheus.Metric {

	type agentMetric struct {
		count   float64
		enabled bool
		status  string
	}

	m := make(map[string]agentMetric)

	for _, agent := range metricContext.agents {
		var agentState = strconv.FormatBool(agent.Enabled) + agent.Status // looks like "trueOnline"

		// Does the state already exist in the map?
		// If it does increase the count on the value else create a new value
		// assign the value back to the map
		metric, ok := m[agentState]
		if ok {
			metric.count++
		} else {
			metric = agentMetric{count: 1, enabled: agent.Enabled, status: agent.Status}
		}

		m[agentState] = metric
	}

	promMetrics := []prometheus.Metric{}
	for _, p := range m {

		promMetric := prometheus.MustNewConstMetric(
			installedBuildAgentsDesc,
			prometheus.GaugeValue,
			p.count,
			strconv.FormatBool(p.enabled),
			p.status,
			metricContext.pool.Name)

		promMetrics = append(promMetrics, promMetric)
	}
	return promMetrics
}
