# Distributed Job Scheduler - Makefile
# Supports both Docker and Podman

DEPLOY_FOLDER = deploy

# Docker commands
DOCKER_COMPOSE = docker-compose
DOCKER = docker

# Podman commands  
PODMAN_COMPOSE = podman-compose
PODMAN = podman

# Default to docker
DEFAULT_ENGINE = docker

.PHONY: help up down logs clean restart status docker-up docker-down podman-up podman-down submit-test-jobs

# Help target
help:
	@echo "Distributed Job Scheduler - Available Commands"
	@echo "=============================================="
	@echo ""
	@echo "Docker Commands:"
	@echo "  make docker-up       - Start services with Docker"
	@echo "  make docker-down     - Stop services with Docker"
	@echo "  make docker-logs     - Show Docker service logs"
	@echo "  make docker-clean    - Clean Docker resources"
	@echo ""
	@echo "Podman Commands:"
	@echo "  make podman-up       - Start services with Podman"
	@echo "  make podman-down     - Stop services with Podman"
	@echo "  make podman-logs     - Show Podman service logs"
	@echo "  make podman-clean    - Clean Podman resources"
	@echo ""
	@echo "Generic Commands (uses $(DEFAULT_ENGINE)):"
	@echo "  make up              - Start services"
	@echo "  make down            - Stop services"
	@echo "  make restart         - Restart services"
	@echo "  make logs            - Show service logs"
	@echo "  make status          - Show service status"
	@echo "  make clean           - Clean resources"

# Docker-specific commands
docker-up:
	@echo "Starting services with Docker..."
	cd $(DEPLOY_FOLDER) && $(DOCKER_COMPOSE) up --build -d
	@echo "Services started! Check status with 'make docker-status'"

docker-down:
	@echo "Stopping Docker services..."
	cd $(DEPLOY_FOLDER) && $(DOCKER_COMPOSE) down

docker-logs:
	cd $(DEPLOY_FOLDER) && $(DOCKER_COMPOSE) logs -f

docker-clean:
	@echo "Cleaning Docker resources..."
	cd $(DEPLOY_FOLDER) && $(DOCKER_COMPOSE) down -v --rmi all
	$(DOCKER) system prune -f
	$(DOCKER) volume prune -f

docker-status:
	@echo "Docker Services Status:"
	$(DOCKER) ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# Podman-specific commands
podman-up:
	@echo "Starting services with Podman..."
	$(PODMAN) volume rm -a 2>/dev/null || true
	cd $(DEPLOY_FOLDER) && $(PODMAN_COMPOSE) up --build -d
	@echo "Services started! Check status with 'make podman-status'"

podman-down:
	@echo "Stopping Podman services..."
	cd $(DEPLOY_FOLDER) && $(PODMAN_COMPOSE) down

podman-logs:
	cd $(DEPLOY_FOLDER) && $(PODMAN_COMPOSE) logs -f

podman-clean:
	@echo "Cleaning Podman resources..."
	cd $(DEPLOY_FOLDER) && $(PODMAN_COMPOSE) down -v
	$(PODMAN) system prune -f
	$(PODMAN) volume prune -f

podman-status:
	@echo "Podman Services Status:"
	$(PODMAN) ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" --filter "name=dts_"

# Generic commands (use default engine)
ifeq ($(DEFAULT_ENGINE),docker)
up: docker-up
down: docker-down  
logs: docker-logs
clean: docker-clean
status: docker-status
ENGINE_COMPOSE = $(DOCKER_COMPOSE)
ENGINE = $(DOCKER)
else
up: podman-up
down: podman-down
logs: podman-logs  
clean: podman-cleandeploy_pgdata
status: podman-status
ENGINE_COMPOSE = $(PODMAN_COMPOSE)
ENGINE = $(PODMAN)
endif

# Common commands
restart:
	@echo "Restarting services..."
	$(MAKE) down
	sleep 2
	$(MAKE) up

dev:
	@echo "Starting in development mode (with logs)..."
	cd $(DEPLOY_FOLDER) && $(ENGINE_COMPOSE) build --no-cache && $(ENGINE_COMPOSE) up


# Quick job submission for testing
submit-test-jobs:
	@echo "Submitting test jobs..."
	curl -X POST http://localhost:8000/submit_job -H "Content-Type: application/json" -d '{"name": "cpu_intensive", "payload": "test_cpu_1"}'
	curl -X POST http://localhost:8000/submit_job -H "Content-Type: application/json" -d '{"name": "io_intensive", "payload": "test_io_1"}'
	curl -X POST http://localhost:8000/submit_job -H "Content-Type: application/json" -d '{"name": "mixed_workload", "payload": "test_mixed_1"}'
	@echo "Test jobs submitted!"

# Show service URLs
urls:
	@echo "Service URLs:"
	@echo "  Submitter API:      http://localhost:8000"
	@echo "  Coordinator:        http://localhost:9000"
	@echo "  Grafana Dashboard:  http://localhost:3000 (admin/admin)"
	@echo "  Prometheus:         http://localhost:9090"
	@echo "  Worker 1:           http://localhost:7001"
	@echo "  Worker 2:           http://localhost:7002"  
	@echo "  Worker 3:           http://localhost:7003"


# Cleanup shortcuts
clean-all: clean
	@echo "Cleaning..."
	$(ENGINE) system prune -af
	$(ENGINE) volume prune -f
	$(ENGINE) network prune -f



# Override default engine with environment variable
# Usage: ENGINE=podman make up
ifdef ENGINE
DEFAULT_ENGINE = $(ENGINE)
endif