package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

func distributeJobs() {
	for {
		// BRPop (Blocking Right Pop), 0 -> wait forever until a new item is available
		result, err := redisClient.BRPop(0, "job_queue").Result()
		if err != nil {
			fmt.Println("Error: ", err)
			continue
		}
		jobJson := result[1]
		fmt.Println("Received job:", jobJson)

		// Parse the job
		var job Job
		if err := json.Unmarshal([]byte(jobJson), &job); err != nil {
			fmt.Println("Error parsing job: ", err)
			continue
		}

		// Select a worker from available workers (LRU Logic)
		workerUrl, err := selectWorkerAndLeaseJob(job)
		if workerUrl == "" || err != nil {
			fmt.Println("No available worker found, requeueing job after delay")
			time.Sleep(5 * time.Second)
			redisClient.LPush("job_queue", jobJson)
			continue
		}

		go sendJobToWorker(workerUrl, job)

	}
}

func selectWorkerAndLeaseJob(job Job) (string, error) {
	tx, err := db.Begin()

	if err != nil {
		fmt.Println("Error starting transaction: ", err)
		return "", err
	}

	defer tx.Rollback()

	// Select Least Recently used worker
	var workerUrl string

	// Row level lock
	err = tx.QueryRow(`
		SELECT url FROM workers
		WHERE state = 'available'
		ORDER BY jobs_completed ASC, url ASC
		LIMIT 1
		FOR UPDATE
	`).Scan(&workerUrl)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // no available workers found
		}
		fmt.Println("Error selecting workers", err)
		return "", err
	}

	// Update job status to leased and update leasing information
	_, err = tx.Exec(`
		UPDATE jobs
		SET status = $1, lease_start = NOW(), lease_timeout = $2, leased_to_worker = $3
		WHERE id = $4
		`, "leased", 20, workerUrl, job.ID)

	if err != nil {
		fmt.Println("Error leasing job:", job.ID, err)
		return "", err
	}

	// Update worker status as busy
	_, err = tx.Exec(`
		UPDATE workers
		SET state = 'busy' where url = $1
	`, workerUrl)

	if err != nil {
		fmt.Println("Error marking worker as busy:", workerUrl, err)
		return "", err
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		fmt.Println("Error committing transaction:", err)
		return "", err
	}

	fmt.Println("Leased job: ", job.ID, "to worker: ", workerUrl)
	return workerUrl, nil
}

func processJobResults() {
	for {
		// Listen for job results
		result, err := redisClient.BRPop(0, "job_results").Result()

		if err != nil {
			fmt.Println("Error getting job result:", err)
			continue
		}

		resultJson := result[1]
		fmt.Println("Received job result:", resultJson)

		// Parse the result
		var jobResult map[string]interface{}

		if err := json.Unmarshal([]byte(resultJson), &jobResult); err != nil {
			fmt.Println("Error parsing job result:", err)
			continue
		}

		// Update database based on job result
		jobID := jobResult["job_id"].(string)
		status := jobResult["status"].(string)
		workerUrl := jobResult["worker_url"].(string)

		// Check if job is already marked completed by some other worker then ignore the result push
		var dbJobStatus string
		var currentRetries int
		err = db.QueryRow("SELECT status, retries FROM jobs WHERE id = $1", jobID).Scan(&dbJobStatus, &currentRetries)

		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Println("Job not found", jobID)
				continue
			}
			fmt.Println("Error checking job status for job id:", jobID, err)
			continue
		}

		if dbJobStatus == "completed" {
			fmt.Println("Job", jobID, "already completed, ignoring result push by ", workerUrl)
			continue
		}

		if status == "completed" {
			// Record job completion
			jobsTotal.WithLabelValues("completed").Inc()

			// Calculate processing duration given timing info
			if createdAt, ok := jobResult["created_at"]; ok {
				if createdTime, err := time.Parse(time.RFC3339, createdAt.(string)); err == nil {
					duration := time.Since(createdTime).Seconds()
					jobProcessingDuration.WithLabelValues(workerUrl).Observe(duration)
				}
			}

			result := jobResult["result"].(string)
			_, err = db.Exec(
				"UPDATE jobs SET status = $1, completed_at = NOW(), result = $2 WHERE id = $3",
				status, result, jobID,
			)

			if err != nil {
				fmt.Println("Error updating job_id:", jobID, "results in database")
				continue
			}

			fmt.Println("Completed job_id", jobID, "and updated results in database")

		} else {
			// Job Failed
			if currentRetries >= MAX_RETRIES {
				// Record failed job
				jobsTotal.WithLabelValues("failed").Inc()
				retryAttempts.WithLabelValues("max_retries_exceeded").Observe(float64(currentRetries))

				sendToDeadLetterQueue(jobID, jobResult)

				_, err := db.Exec(
					"UPDATE jobs SET status = 'failed', completed_at = NOW() where id = $1", jobID,
				)

				if err != nil {
					fmt.Println("Error marking job as failed for job:", jobID, err)
				}
				fmt.Println("Job:", jobID, "exceeded max retries, sent to DLQ")
			} else {
				// Record retry attempt
				retryAttempts.WithLabelValues("worker_failure").Observe(float64(currentRetries))

				// Retry the job with exponential backoff
				delay := calculateBackoffDelay(currentRetries)
				fmt.Printf("Job %s failed, retrying in %v (attempt %d/%d)\n", jobID, delay, currentRetries+1, MAX_RETRIES)

				go func(id string, retries int, delay time.Duration) {
					time.Sleep(delay)
					requeueFailedJob(id)
				}(jobID, currentRetries, delay)
			}
		}

		updateWorkerJobCount(workerUrl)
		updateWorkerState(workerUrl, "available")

	}
}

func requeueFailedJob(jobID string) {
	// Get job details from database
	var job Job
	err := db.QueryRow("SELECT id, name, payload FROM jobs WHERE id = $1", jobID).Scan(&job.ID, &job.Name, &job.Payload)
	if err != nil {
		fmt.Printf("Error fetching job %s for requeue: %v\n", jobID, err)
		return
	}

	// Update job status to pending and increment retry count
	_, err = db.Exec(
		"UPDATE jobs SET status = 'pending', lease_start = NULL, lease_timeout = NULL, retries = retries + 1 WHERE id = $1",
		jobID,
	)
	if err != nil {
		fmt.Printf("Error updating job %s for requeue: %v\n", jobID, err)
		return
	}

	// Add back to job queue
	jobJson, _ := json.Marshal(job)
	redisClient.LPush("job_queue", jobJson)
	fmt.Printf("Requeued failed job %s\n", jobID)
}

func sendToDeadLetterQueue(jobID string, jobResult map[string]interface{}) {
	// Add metadata to the DQL message
	dqlMessage := map[string]interface{}{
		"job_id":       jobID,
		"original_job": jobResult,
		"failed_at":    time.Now().UTC(),
		"reason":       "max_retries_exceeded",
	}

	dqlJson, _ := json.Marshal(dqlMessage)

	// Push to dead  letter queue in Redis
	err := redisClient.LPush(DLQ_QUEUE, dqlJson).Err()

	if err != nil {
		fmt.Printf("Error sending job %s to DQL: %v\n", jobID, err)
	} else {
		fmt.Printf("Job %s sent to Dead Letter Queue\n", jobID)
	}
}

func processDLQ() {
	// Helper method to process tasks that crossed max retry count due to failures
	for {
		result, err := redisClient.BRPop(60*time.Second, DLQ_QUEUE).Result()
		if err != nil {
			if err == redis.Nil {
				// No dead job requests
				time.Sleep(10 * time.Second)
				continue
			}
			fmt.Println("Error reading from DQL:", err)
			continue
		}

		dqlMessage := result[1]
		fmt.Println("DQL Message:", dqlMessage)
		// Placeholder to add further functionality to escalate the dead job
	}
}

func leaseMonitor() {
	for {
		rows, err := db.Query(`
			SELECT id, name, payload, retries FROM jobs
			WHERE status = 'leased'
			AND lease_start + (lease_timeout || ' seconds')::interval < NOW()
		`)

		if err != nil {
			fmt.Println("Error querying for expired leases:", err)
			time.Sleep(10 * time.Second)
			continue
		}

		var expiredJobs []struct {
			Job
			Retries int
		}

		for rows.Next() {
			var job struct {
				Job
				Retries int
			}
			if err := rows.Scan(&job.ID, &job.Name, &job.Payload, &job.Retries); err != nil {
				fmt.Println("Error scanning job row:", err)
				continue
			}
			expiredJobs = append(expiredJobs, job)
		}

		rows.Close()

		for _, expiredJob := range expiredJobs {
			// Record lease timeout
			leaseTimeouts.Inc()

			if expiredJob.Retries >= MAX_RETRIES {
				// Record timeout job sent to DLQ
				jobsTotal.WithLabelValues("timeout").Inc()
				retryAttempts.WithLabelValues("lease_timeout").Observe(float64(expiredJob.Retries))

				// Send to dead letter queue
				jobResult := map[string]interface{}{
					"job_id":     expiredJob.ID,
					"name":       expiredJob.Name,
					"payload":    expiredJob.Payload,
					"status":     "timeout",
					"error":      "Job lease expired - max retries exceeded",
					"worker_url": "",
				}
				sendToDeadLetterQueue(expiredJob.ID, jobResult)

				// Mark as failed
				_, err := db.Exec(
					"UPDATE jobs SET status = 'failed', completed_at = NOW() WHERE id = $1",
					expiredJob.ID,
				)

				if err != nil {
					fmt.Println("Error marking expired job as failed: ", err)
				}

				fmt.Printf("Expired job %s sent to DLQ after %d retries\n", expiredJob.ID, expiredJob.Retries)
			} else {
				// Record retry for timeout
				retryAttempts.WithLabelValues("lease_timeout").Observe(float64(expiredJob.Retries))

				// Retry the job with exponential backoff
				delay := calculateBackoffDelay(expiredJob.Retries)
				fmt.Printf("Job %s lease expired, retrying in %v (attempt %d/%d)\n", expiredJob.ID, delay, expiredJob.Retries+1, MAX_RETRIES)

				// Update job status
				_, err := db.Exec(
					"UPDATE jobs SET status = 'pending', lease_start = NULL, lease_timeout = NULL, retries = retries + 1 WHERE id = $1",
					expiredJob.ID,
				)
				if err != nil {
					fmt.Println("Unable to update lease timeout job to database, job_id:", expiredJob.ID, err)
					continue
				}

				// Schedule requeue with delay
				go func(job Job, delay time.Duration) {
					time.Sleep(delay)
					jobJson, _ := json.Marshal(job)
					redisClient.LPush("job_queue", jobJson)
					fmt.Printf("Requeued expired job %s after backoff\n", job.ID)
				}(expiredJob.Job, delay)

			}

		}

		time.Sleep(10 * time.Second)
	}
}
