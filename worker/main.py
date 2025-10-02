from fastapi import FastAPI, Request
import httpx
import os
import asyncio
import redis
import json
import random
import hashlib
import time
from contextlib import asynccontextmanager
from concurrent.futures import ThreadPoolExecutor

WORKER_URL = os.getenv("WORKER_URL")
COORDINATOR_URL = os.getenv("COORDINATOR_URL")
REDIS_ADDR = os.getenv("REDIS_ADDR")

redis_client = redis.Redis.from_url(REDIS_ADDR)
executor = ThreadPoolExecutor(max_workers=2)

@asynccontextmanager
async def lifespan(app: FastAPI):
    print(f"Worker starting up. WORKER_URL: {WORKER_URL}, COORDINATOR_URL: {COORDINATOR_URL}")
    # Self register with coordinator

    registered = False
    retry_count = 0
    max_retries = 10

    while not registered and retry_count < max_retries:
        try:
            print(f"Attempting to register with coordinator at {COORDINATOR_URL}")
            async with httpx.AsyncClient() as client:
                resp = await client.post(f"{COORDINATOR_URL}/register_worker", json={"worker_url": WORKER_URL})
                print(f"Registered worker with coordinator at {COORDINATOR_URL}, response status: {resp.status_code}")
                registered = True
        except Exception as e:
            retry_count += 1
            print(f"Failed to register with coordinator (attempt {retry_count}): {e}")

            if retry_count < max_retries:
                print("Retrying in 3 seconds...")
                await asyncio.sleep(3)
            else:
                print("Max retries reached. Starting worker without registration...")

    # start heartbeat background task
    print("Starting heartbeat task...")
    heartbeat_task = asyncio.create_task(send_heartbeat())

    yield

    heartbeat_task.cancel()

app = FastAPI(lifespan=lifespan)

async def send_heartbeat():
    """
    Method to send heartbeat to Redis Message queue
    """
    while True:
        redis_client.set(f"worker:{WORKER_URL}", "alive", ex=30)
        print(f"Heartbeat sent for {WORKER_URL}")
        await asyncio.sleep(10)


@app.post('/run_job')
async def run_job(request: Request):
    data = await request.json()
    job_id = data.get('job_id')
    job_name = data.get('name', 'default')
    print(f"Received job_id : {job_id}, job_name: {job_name}")

    # Payload validator
    payload_content = data.get('payload')

    try:
        # Example to handle invalid payload content
        if 'invalid_job' in payload_content:
            # Push failed job to redis result queue
            result = {
                "job_id": job_id,
                "status": "failed",
                "error": "Invalid job content",
                "worker_url": WORKER_URL
            }

            redis_client.lpush("job_results", json.dumps(result))
            return {"status": "failed", "message": "Invalid job content"}

        # Simulate valid job processing


        start_time = time.time()


        if job_name == "cpu_intensive":
            result_data = await simulate_cpu_work()
        elif job_name == "io_intensive":
            result_data = await simulate_io_work()
        elif job_name == "mixed_workload":
            result_data  = await simulate_mixed_work()
        elif job_name == "network_task":
            result_data = await simulate_network_work()
        else:
            # Default case
            result_data = await simulate_variable_work()

        processing_time = time.time() - start_time

        print(f"Job {job_id} completed in {processing_time:.2f}s")

        # Push successful job to redis
        result = {
            "job_id": job_id,
            "status": "completed",
            "result": f"Job {job_id} processed successfully | result_data: {result_data}",
            "processing_time": processing_time,
            "worker_url": WORKER_URL
        }

        redis_client.lpush("job_results", json.dumps(result))
        return {"status": "completed", "message": f"Job {job_id} completed"}

    except Exception as e:
        # Push failed result on exception
        result = {
            "job_id": job_id,
            "status": "failed",
            "error": str(e),
            "worker_url": WORKER_URL
        }
        redis_client.lpush("job_results", json.dumps(result))

        return {"status": "failed", "message": str(e)}


# Job Processing simulation methods
async def simulate_cpu_work():
    """
    CPU Intensive Task: Hash Computation
    """

    def cpu_task():
        data = "simulate_cpu_work" * 10000
        res = ""
        for i in range(random.randint(5000, 15000)):
            hashlib.sha256(f"{data}_{i}".encode()).hexdigest()
        return f"Computed {i + 1} hashes"

    # Run CPU work in thread pool to avoid blocking
    loop = asyncio.get_event_loop()
    return await loop.run_in_executor(executor, cpu_task)

async def simulate_io_work():
    """ I/O intensive task: File operations"""
    filename = f"/tmp/worker_job_{random.randint(1000, 9999)}.txt"

    # write a large file
    with open(filename, 'w') as f:
        for i in range(random.randint(1000, 9999)):
            f.write(f"Line {i}: Placeholder data for I/O work simulation\n")

    # Read and process file
    line_count = 0
    with open(filename, 'r') as f:
        for line in f:
            line_count += 1

    os.remove(filename)
    return f"Processed {line_count} lines from file: {filename}"

async def simulate_mixed_work():
    """Mixed CPU + I/O Work"""
    # Some CPU work
    cpu_result = await simulate_cpu_work()

    # Some I/O work
    io_work = await simulate_io_work()

    return f"Mixed work: CPU Work: {cpu_result} | IO Work : {io_work}"

async def simulate_network_work():
    """Network intensive work"""
    try:
        # Simulating API calls
        async with httpx.AsyncClient as client:
            tasks = []
            for i in range(random.randint(1, 5)):
                # Create multiple concurrent requests
                task = client.get(f"https://httpbin.org.delay/{random.randint(1, 5)}")
                tasks.append(task)

            responses = await asyncio.gather(*tasks, return_exceptions=True)
            successful_requests = sum(1 for r in responses if not isinstance(r, Exception))
            
        return f"Network task: {successful_requests}/{len(tasks)} requests successful"
    
    except Exception as e:
        return f"Network task failed: {str(e)}"

async def simulate_variable_work():
    """Variable duration work with differnt speeds"""
    work_type = random.choice(['fast', 'medium', 'slow'])
    
    if work_type == 'fast':
        duration = random.uniform(0.1, 1.0)
    elif work_type == 'medium':
        duration = random.uniform(1.0, 10.0)
    elif work_type == 'slow':
        duration = random.uniform(10.0, 50.0)
        
    await asyncio.sleep(duration)
    
    return f"{work_type} task completed in {duration:.2f}"