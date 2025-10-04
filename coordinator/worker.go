package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func updateWorkerState(workerUrl string, state string) error {
	// Method to update worker state in database
	res, err := db.Exec(
		"UPDATE workers SET state = $1 WHERE url = $2",
		state, workerUrl,
	)

	if err != nil {
		fmt.Println("Error updating worker status for workerURL:", workerUrl, "to state:", state)
		return err
	}

	rows, _ := res.RowsAffected()
	fmt.Println("updatedWorkerState: set workerUrl: ", workerUrl, "to state: ", state, "rows affected:", rows)

	return nil

}

func workerHeartbeatVerifier() {
	for {
		// Get list of all workers from worker table in db
		rows, err := db.Query("SELECT url, state FROM workers")

		if err != nil {
			fmt.Println("Error fetching all worker urls")
			continue
		}

		var workers []Worker

		for rows.Next() {
			var w Worker
			if err := rows.Scan(&w.URL, &w.state); err != nil {
				fmt.Println("Error scanning row:", err)
				continue
			}
			workers = append(workers, w)
		}

		rows.Close()

		if len(workers) == 0 {
			fmt.Println("No workers found in database")
			time.Sleep(10 * time.Second)
			continue
		}
		// MGET to fetch all keys from redis
		keys := make([]string, len(workers))

		for i, w := range workers {
			keys[i] = "worker:" + w.URL
		}

		vals, err := redisClient.MGet(keys...).Result()

		for err != nil {
			fmt.Println("Error in MGET:", err)
			time.Sleep(10 * time.Second)
			continue
		}

		// For each worker in workers verify key worker:{WORKER_URL} is available and update state accordingly
		for i, w := range workers {
			val := vals[i]

			if val == nil {
				// Key does not exist -> worker is unavailable
				if w.state != "unavailable" {
					fmt.Println("Unavailable worker found :", w.URL)
					updateWorkerState(w.URL, "unavailable")
				}
			} else {
				// Key exists -> worker is available
				if w.state == "unavailable" {
					fmt.Println("Found heartbeat of an unavailable worker", w.URL)
					updateWorkerState(w.URL, "available")
				}
			}
		}

		time.Sleep(10 * time.Second)
	}
}

func sendJobToWorker(workerUrl string, job Job) {
	// Request payload
	jobPayload := map[string]interface{}{
		"job_id":  job.ID,
		"name":    job.Name,
		"payload": job.Payload,
	}

	payloadBytes, err := json.Marshal(jobPayload)
	if err != nil {
		fmt.Println("Error marshalling job payload:", err)
		return
	}

	// Create http client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Mark worker as busy before sending Job Request
	// Single job -> busy state is for simulation
	updateWorkerState(workerUrl, "busy")

	fmt.Println("DEBUG: sending payload Payload: ", jobPayload)
	// Send POST request
	resp, err := client.Post(
		workerUrl+"/run_job",
		"application/json",
		bytes.NewBuffer(payloadBytes),
	)

	if err != nil {
		fmt.Println("Error sending job to worker", workerUrl, err)
		// Mark worker as unavailable
		updateWorkerState(workerUrl, "unavailable")
		// Requeue the job
		jobJson, _ := json.Marshal(job)
		redisClient.LPush("job_queue", jobJson)
		return
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response from worker", workerUrl, err)
		return
	}

	if resp.StatusCode == 200 {
		fmt.Println("Successfully sent job:", job.ID, "to worker:", workerUrl, "response: ", string(body))
	} else {
		fmt.Println("worker: ", workerUrl, "returned error for job: ", job.ID, "status:", resp.StatusCode, "body:", string(body))
		// Requeue the job if worker rejected
		time.Sleep(5 * time.Second)
		jobJson, _ := json.Marshal(job)
		redisClient.LPush("job_queue", jobJson)
	}
}

func updateWorkerJobCount(workerUrl string) {
	_, err := db.Exec(
		"UPDATE workers SET jobs_completed = jobs_completed + 1 WHERE url = $1",
		workerUrl,
	)

	if err != nil {
		fmt.Println("Error updaing job count for worker:", workerUrl, "err:", err)
	}

}
