# Stage 1: Build the Go application
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies if needed by go mod
RUN apk add --no-cache git

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build the batch application
# Adjust the output path if your main.go is elsewhere or you want a specific name
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/batch_add_questions ./cmd/batch_add_questions/main.go

# Stage 2: Create the runtime image
FROM alpine:latest

WORKDIR /app

# Create a non-root user and group for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy the built binary from the builder stage
COPY --from=builder /app/batch_add_questions /app/batch_add_questions

# Copy configuration files (if any are needed at runtime and not solely managed by env vars)
# Assuming config.yaml might be used or serve as a base for env vars.
# Adjust if your config is purely env-based.
COPY configs/ /app/configs/
# Ensure the appuser can read the config if it's file-based
RUN chown -R appuser:appgroup /app/configs

# Set the user for the container
USER appuser

# Command to run the application
# Pass environment variables at runtime (e.g., via Kubernetes CronJob spec or docker run -e)
ENTRYPOINT ["/app/batch_add_questions"]
