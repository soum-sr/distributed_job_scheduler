#!/bin/bash

echo "=== High Volume Stress Test ==="
echo "This will submit 200 jobs in waves..."

# Configuration
TOTAL_JOBS=200
BATCH_SIZE=25
DELAY_BETWEEN_BATCHES=2

job_types=("cpu_intensive" "io_intensive" "mixed_workload" "network_task" "default")
payloads=("cpu_task" "io_task" "mixed_task" "network_task" "variable_task")

submitted=0
batch=1

while [ $submitted -lt $TOTAL_JOBS ]; do
    echo ""
    echo "--- Batch $batch (Jobs $((submitted + 1)) - $((submitted + BATCH_SIZE))) ---"
    
    for i in $(seq 1 $BATCH_SIZE); do
        if [ $submitted -ge $TOTAL_JOBS ]; then
            break
        fi
        
        # Randomly select job type
        job_index=$((RANDOM % 5))
        job_type=${job_types[$job_index]}
        payload=${payloads[$job_index]}
        
        curl -s -X POST http://localhost:8000/submit_job \
            -H "Content-Type: application/json" \
            -d "{\"name\": \"$job_type\", \"payload\": \"${payload}_batch${batch}_${i}\"}" \
            --max-time 2 > /dev/null &
        
        submitted=$((submitted + 1))

        sleep 0.05
    done
    
    wait 
    echo "> Batch $batch completed ($submitted/$TOTAL_JOBS jobs submitted)"
    
    if [ $submitted -lt $TOTAL_JOBS ]; then
        echo "> Waiting ${DELAY_BETWEEN_BATCHES}s before next batch..."
        sleep $DELAY_BETWEEN_BATCHES
    fi
    
    batch=$((batch + 1))
done

echo ""
echo "> Stress test completed!"
echo "> Total jobs submitted: $submitted"
echo "> Monitor the system in Grafana: http://localhost:3000"


