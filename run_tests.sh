#!/bin/bash

echo "Starting test environment with PostgreSQL and Redis..."

echo "Running tests with environment variables..."
go test -v ./...