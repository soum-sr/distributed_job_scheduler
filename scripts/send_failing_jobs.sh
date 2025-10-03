#!/bin/bash

echo "=== Failure Scenario Testing ==="

# Test invalid jobs (should go to DLQ after retries)
echo "1. Testing invalid jobs..."
for i in {1..3}; do
    curl -s -X POST http://localhost:8000/submit_job \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"test_job\", \"payload\": \"invalid_job_content_$i\"}" \
        --max-time 5
    echo "Submitted invalid job $i"
done

sleep 2

# Test normal jobs mixed with invalid
echo ""
echo "2. Testing mixed valid/invalid jobs..."
for i in {1..10}; do
    if [ $((i % 3)) -eq 0 ]; then
        # Every 3rd job is invalid
        payload="invalid_job_mixed_$i"
        echo "Submitting invalid job $i"
    else
        payload="valid_task_$i"
        echo "Submitting valid job $i"
    fi
    
    curl -s -X POST http://localhost:8000/submit_job \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"default\", \"payload\": \"$payload\"}" \
        --max-time 5 > /dev/null &
done

wait
echo "> Failure scenario testing completed!"
echo "> Check DLQ metrics in Grafana"