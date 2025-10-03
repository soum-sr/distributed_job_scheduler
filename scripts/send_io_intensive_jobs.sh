
#!/bin/bash

echo "=== Testing IO Intensive Jobs ==="
echo "Submitting 10 IO intensive jobs..."

for i in {1..10}; do
    curl -s -X POST http://localhost:8000/submit_job \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"io_intensive\", \"payload\": \"io_heavy_task_$i\"}" \
        --max-time 5
    
    echo "Submitted IO job $i"
    sleep 0.1
done

echo "> All IO intensive jobs submitted!"