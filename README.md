# KubeRLy Test Example Repository

This repository demonstrates a complete testing pipeline for a Go application with Docker, Docker Compose, PostgreSQL, and Redis.

## Repository Structure

```
run-tests-example/
├── main.go                 # Main Go application (HTTP server with DB integration)
├── Dockerfile              # Multi-stage Docker build
├── docker-compose.yaml     # Services: app, postgres, redis
├── example_test.go         # Integration tests for app + databases
├── go.mod                  # Go module dependencies
├── go.sum                  # Go module checksums
├── Makefile                # Build and test targets
├── run_tests.sh            # Complete test pipeline script
└── README.md               # This file
```

## Features

- **Go HTTP Server**: REST API with health check, test data, and cache operations
- **Database Integration**: PostgreSQL for persistent storage with Redis caching
- **Docker Support**: Multi-stage Docker build for optimized production images
- **Complete Test Pipeline**: Build → Docker Build → Test → Cleanup
- **Integration Testing**: Tests verify app + database + cache integration
- **Makefile Targets**: Easy commands for development and testing

## Application Endpoints

The Go application provides these HTTP endpoints:

- `GET /` - Root endpoint with available routes
- `GET /health` - Health check with database and cache status
- `GET /api/test` - Retrieve test data from PostgreSQL
- `GET /api/data` - Get data with Redis caching (shows cache HIT/MISS)
- `POST /api/data` - Insert new data and invalidate cache
- `GET /api/cache?key=<key>` - Retrieve value from Redis cache
- `POST /api/cache` - Set value in Redis cache with TTL

## Quick Start

### Prerequisites

- Go 1.24+
- Docker and Docker Compose
- Make (optional, for using Makefile targets)

### Running Tests

#### Option 1: Using the script (Recommended)
```bash
chmod +x run_tests.sh
./run_tests.sh
```

#### Option 2: Using Make
```bash
make test-pipeline
```

#### Option 3: Manual steps
```bash
# Build Go app
go build -o bin/app .

# Build Docker image
docker build -t kuberly-test-app .

# Start services
docker-compose up -d

# Wait for services to be ready
sleep 20

# Set environment variables and run tests
export APP_HOST=localhost APP_PORT=8080
export POSTGRES_HOST=localhost POSTGRES_PORT=5432
export POSTGRES_USER=postgres POSTGRES_PASSWORD=postgres POSTGRES_DB=testdb
export REDIS_HOST=localhost REDIS_PORT=6379
go test -v ./...

# Cleanup
docker-compose down
```

## Test Pipeline

The complete test pipeline includes:

1. **Build**: Compile the Go application
2. **Docker Build**: Create Docker image
3. **Service Startup**: Start PostgreSQL, Redis, and app services
4. **Integration Testing**: Run tests against all services
   - PostgreSQL connectivity and operations
   - Redis connectivity and operations
   - Application HTTP endpoints
   - Database + cache integration
5. **Cleanup**: Stop all services and clean up

## Makefile Targets

- `make build` - Build Go application
- `make test` - Run tests locally (without Docker)
- `make docker-build` - Build Docker image
- `make docker-run` - Run Docker container
- `make docker-test` - Run tests with Docker Compose
- `make test-integration` - Test only application integration
- `make test-pipeline` - Complete pipeline (build + test)
- `make all` - Clean, build, and test everything
- `make dev-start` - Start development environment
- `make dev-stop` - Stop development environment
- `make dev-logs` - View service logs
- `make dev-shell` - Access app container shell

## Environment Variables

The application uses these environment variables:

- `PORT` - HTTP server port (default: 8080)
- `POSTGRES_HOST` - PostgreSQL host (default: postgres)
- `POSTGRES_PORT` - PostgreSQL port (default: 5432)
- `POSTGRES_USER` - PostgreSQL user (default: postgres)
- `POSTGRES_PASSWORD` - PostgreSQL password (default: postgres)
- `POSTGRES_DB` - PostgreSQL database (default: testdb)
- `REDIS_HOST` - Redis host (default: redis)
- `REDIS_PORT` - Redis port (default: 6379)

## Development

To run the application locally:

```bash
make run
# or
go run main.go
```

To run tests locally (without Docker):

```bash
make test
# or
go test -v ./...
```

To start development environment:

```bash
make dev-start
make dev-logs  # View logs
make dev-shell # Access container
make dev-stop  # Stop services
```

## Docker Commands

```bash
# Build image
docker build -t kuberly-test-app .

# Run container
docker run -p 8080:8080 kuberly-test-app

# Run with environment variables
docker run -p 8080:8080 -e PORT=9000 kuberly-test-app
```

## Testing Strategy

The repository demonstrates a comprehensive testing approach:

1. **Unit Tests**: Test individual functions and methods
2. **Integration Tests**: Test database connectivity and operations
3. **Application Tests**: Test HTTP endpoints and business logic
4. **End-to-End Tests**: Test complete application with all services
5. **Cache Testing**: Verify Redis caching behavior (HIT/MISS patterns)

## Use Cases

This repository is perfect for:

- **CI/CD Pipelines**: Automated testing with Docker
- **Development Workflows**: Local development with full stack
- **Learning**: Understanding Go + Docker + Database integration
- **Prototyping**: Quick setup for new projects
- **Testing**: Demonstrating testing best practices

This repository serves as a demonstration of how to set up a complete testing environment for Go applications with Docker and Docker Compose, perfect for CI/CD pipelines and development workflows.
