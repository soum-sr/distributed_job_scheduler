package main

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	jobsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dts_jobs_total",
			Help: "Total number of jobs procesed by status",
		},
		[]string{"status"}, // completed, failed, timeout
	)

	workersActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dts_workers_active",
			Help: "Number of workers by state",
		},
		[]string{"state"}, // available, busy, unavailable
	)

	jobsInQueue = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "dts_jobs_in_queue",
			Help: "Number of jobs waiting in queue",
		},
	)

	jobsInDLQ = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "dts_jobs_in_dlq",
			Help: "Number of jobs in dead letter queue",
		},
	)

	jobProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dts_job_processing_duration_seconds",
			Help:    "Time spent processing jobs",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
		},
		[]string{"worker_url"},
	)

	retryAttempts = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dts_job_retry_attempts",
			Help:    "Number of retry attempts per job",
			Buckets: []float64{0, 1, 2, 3, 4, 5},
		},
		[]string{"reason"}, // timeout, failure, worker_error
	)

	leaseTimeouts = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dts_lease_timeouts_total",
			Help: "Total number of job lease timeouts",
		},
	)
)

func updateMetrics() {
	for {
		rows, err := db.Query("SELECT state, COUNT(*) FROM workers GROUP BY state")
		if err == nil {
			// Reset all states to 0
			workersActive.WithLabelValues("available").Set(0)
			workersActive.WithLabelValues("busy").Set(0)
			workersActive.WithLabelValues("unavailable").Set(0)

			for rows.Next() {
				var state string
				var count float64
				if err := rows.Scan(&state, &count); err == nil {
					workersActive.WithLabelValues(state).Set(count)
				}
			}
			rows.Close()
		}

		// Update jobs in queue
		queueLength := redisClient.LLen("job_queue").Val()
		jobsInQueue.Set(float64(queueLength))

		// Update jobs in DLQ
		dlqLength := redisClient.LLen(DLQ_QUEUE).Val()
		jobsInDLQ.Set(float64(dlqLength))

		time.Sleep(10 * time.Second)

	}
}
