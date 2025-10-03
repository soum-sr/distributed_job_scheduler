
#!/bin/bash

echo "=== Testing Network Intensive Jobs ==="
echo "Submitting 10 Network intensive jobs..."

for i in {1..10}; do
    curl -s -X POST http://localhost:8000/submit_job \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"network_task\", \"payload\": \"api_calls_task_$i\"}" \
        --max-time 5
    
    echo "Submitted Network job $i"
    sleep 0.1
done

echo "> All Network intensive jobs submitted!"