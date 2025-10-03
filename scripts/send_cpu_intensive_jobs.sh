
#!/bin/bash

echo "=== Testing CPU Intensive Jobs ==="
echo "Submitting 10 CPU intensive jobs..."

for i in {1..10}; do
    curl -s -X POST http://localhost:8000/submit_job \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"cpu_intensive\", \"payload\": \"cpu_heavy_task_$i\"}" \
        --max-time 5
    
    echo "Submitted CPU job $i"
    sleep 0.1
done

echo "> All CPU intensive jobs submitted!"