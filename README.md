# Quiz Byte Backend

## Overview
Quiz Byte is a backend system that provides a variety of quizzes in computer science and IT. It is built with Go, Oracle DB, GORM, and the Fiber framework.

## Features
- Category and subcategory-based quiz delivery
- Supports multiple-choice and descriptive quizzes
- LLM (Large Language Model)-based auto-grading
- RESTful API
- Integration tests and DB migration support

## Project Structure
```
cmd/                # Entry points (main.go, etc.)
internal/           # Core domain, service, handler, DB, logger, etc.
  domain/           # Domain models
  repository/       # DB models and queries
  handler/          # HTTP handlers
  service/          # Business logic
  logger/           # Logging
  database/         # DB connection/migration
  dto/              # API DTOs
configs/            # Config files
config/             # Environment config
tests/integration/  # Integration tests and sample data
pkg/                # External packages
```

## Development Environment
- Go 1.20+
- Oracle DB (local/remote)
- Docker, Docker Compose (for tests/local dev)
- (Optional) Oracle Instant Client

## Getting Started
1. Install dependencies
```bash
go mod tidy
```
2. Set environment variables or edit config/config.yaml
3. Run DB migration
```bash
go run cmd/migrate/main.go
```
4. Start the server
```bash
go run cmd/api/main.go
```
5. Run integration tests
```bash
go test ./tests/integration
```

## Example Environment Variables
```
DB_USER=system
DB_PASSWORD=oracle
DB_HOST=localhost
DB_PORT=1521
DB_SERVICE_NAME=FREE
```

## Contribution & Contact
- PRs and issues are welcome
- Contact: jaeyeong.i.dev@gmail.com