package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/lib/pq"
)

var db *sql.DB
var redisClient *redis.Client

type Worker struct {
	URL   string
	state string
}

type Job struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Payload string `json:"payload"`
}

const (
	MAX_RETRIES = 3
	DLQ_QUEUE   = "dead_letter_queue"
)

func main() {
	var err error

	// Connect to postgres database
	dbUrl := os.Getenv("DATABASE_URL")

	if dbUrl == "" {
		panic("DB URL not found")
	}

	// Try 10 times, with 3s sleep to connect to database
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", dbUrl)

		if err == nil {
			err = db.Ping()
			if err == nil {
				// Connection successful
				break
			}
		}

		fmt.Println("Waiting for database to be ready...")
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		panic(err)
	}

	// Connect to redis
	redisAddr := os.Getenv("REDIS_ADDR")

	if redisAddr == "" {
		panic("Redis URL not found")
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	if err := redisClient.Ping().Err(); err != nil {
		panic("Could not connect to Redis: " + err.Error())
	}

	// HTTP Handlers
	go func() {
		http.HandleFunc("/register_worker", registerWorkerHandler)

		// Prometheus metrics endpoint
		http.Handle("/metrics", promhttp.Handler())

		fmt.Println("Coordinator HTTP server running on :9000")
		fmt.Println("Prometheus metrics running on :9000/metrics")

		log.Fatal(http.ListenAndServe(":9000", nil))
	}()

	// Worker Heartbeat Verifier
	go workerHeartbeatVerifier()

	// Job Distributer
	go distributeJobs()

	// Lease Monitor
	go leaseMonitor()

	// Job Result Processor
	go processJobResults()

	// DLQ Processor
	go processDLQ()

	// Metrics Updater
	go updateMetrics()

	// Block forever
	select {}

}
