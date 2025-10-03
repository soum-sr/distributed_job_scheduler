FROM python:3.11-slim

WORKDIR /app

COPY . .

RUN pip install --no-cache-dir fastapi uvicorn httpx redis

CMD ["sh", "-c", "uvicorn main:app --host 0.0.0.0 --port ${WORKER_PORT:-7000}"]