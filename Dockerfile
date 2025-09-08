# Build stage
FROM public.ecr.aws/docker/library/golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM public.ecr.aws/docker/library/alpine:latest

RUN apk --no-cache add ca-certificates make

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Copy source code for testing
COPY --from=builder /app/ .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]
