#!/bin/bash

echo "Starting KubeRLy Test Pipeline..."

echo "Step 1: Building Go application..."
go build -o bin/app .

if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi
echo "✅ Build successful!"

echo "Step 2: Building Docker image..."
docker build -t kuberly-test-app .

if [ $? -ne 0 ]; then
    echo "❌ Docker build failed!"
    exit 1
fi
echo "✅ Docker build successful!"

echo "Step 3: Starting test environment with Docker Compose..."
docker-compose up -d

echo "Step 4: Waiting for services to be ready..."
sleep 20

echo "Step 5: Running tests with proper environment variables..."
# Set environment variables for tests to connect to the app service
export APP_HOST=localhost
export APP_PORT=8080
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres
export POSTGRES_DB=testdb
export REDIS_HOST=localhost
export REDIS_PORT=6379

go test -v ./...

TEST_EXIT_CODE=$?

echo "Step 6: Cleaning up..."
docker-compose down

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo "🎉 All tests passed successfully!"
else
    echo "❌ Some tests failed!"
fi

exit $TEST_EXIT_CODE