.PHONY: build test clean run docker-build docker-run docker-test test-integration

# Build the Go application
build:
	go build -o bin/app .

# Run tests locally (without Docker)
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Run the application locally
run: build
	./bin/app

# Build Docker image
docker-build:
	docker build -t kuberly-test-app .

# Run Docker container
docker-run: docker-build
	docker run -p 8080:8080 kuberly-test-app

# Run tests with Docker Compose
docker-test:
	docker-compose up -d
	sleep 20
	export APP_HOST=localhost && \
	export APP_PORT=8080 && \
	export POSTGRES_HOST=localhost && \
	export POSTGRES_PORT=5432 && \
	export POSTGRES_USER=postgres && \
	export POSTGRES_PASSWORD=postgres && \
	export POSTGRES_DB=testdb && \
	export REDIS_HOST=localhost && \
	export REDIS_PORT=6379 && \
	go test -v ./...
	docker-compose down

# Test application integration (requires running services)
test-integration:
	export APP_HOST=app && \
	export APP_PORT=8080 && \
	export POSTGRES_HOST=localhost && \
	export POSTGRES_PORT=5432 && \
	export POSTGRES_USER=postgres && \
	export POSTGRES_PASSWORD=postgres && \
	export POSTGRES_DB=testdb && \
	export REDIS_HOST=localhost && \
	export REDIS_PORT=6379 && \
	go test -v -run TestApp ./...

# Full test pipeline: build Docker image, run tests
test-pipeline: docker-build docker-test

# Build and test everything
all: clean build test docker-build docker-test

# Development helpers
dev-start:
	docker-compose up -d

dev-stop:
	docker-compose down

dev-logs:
	docker-compose logs -f

dev-shell:
	docker-compose exec app sh
