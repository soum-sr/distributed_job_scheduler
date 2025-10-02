package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func registerWorkerHandler(w http.ResponseWriter, r *http.Request) {
	// Define the expected payload
	var payload struct {
		WorkerUrl string `json:"worker_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Insert the worker to the worker table in the database
	_, err := db.Exec(
		`INSERT INTO workers (url, state, jobs_completed)
		VALUES ($1, 'available', 0)
		ON CONFLICT (url)
		DO UPDATE SET state='available'`,
		payload.WorkerUrl,
	)

	if err != nil {
		http.Error(w, "Failed to register worker", http.StatusInternalServerError)
		fmt.Println("Error registering worker: ", err)
		return
	}

	fmt.Println("Registered worker:", payload.WorkerUrl)
	w.Write([]byte("Worker registered successfully"))

}
