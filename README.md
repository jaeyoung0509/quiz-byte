# Quiz Byte Backend

## Overview
Quiz Byte is a backend system that provides a variety of quizzes in computer science and IT. It is built with Go, Oracle DB, and follows Clean Architecture principles with Domain-Driven Design.

## Features
- Category and subcategory-based quiz delivery
- Supports multiple-choice and descriptive quizzes
- LLM (Large Language Model)-based auto-grading
- RESTful API with proper error handling
- Caching support with Redis (using Port & Adapter pattern)
- Similarity-based answer evaluation
- Integration tests and DB migration support
- Proper domain error handling

## Architecture
The project follows Clean Architecture principles:
- Domain Layer: Core business logic and interfaces
- Repository Layer: Data access with adapters
- Service Layer: Application use cases
- Handler Layer: HTTP request handling
- Infrastructure Layer: External services (Redis, DB)

## Project Structure
```
cmd/                # Entry points (main.go, etc.)
internal/           
  adapter/          # Infrastructure adapters (Redis, etc.)
  domain/           # Domain models and interfaces
  repository/       # DB models and queries
  handler/          # HTTP handlers
  service/          # Application use cases
  logger/           # Logging
  database/         # DB connection/migration
  dto/              # API DTOs
  middleware/       # HTTP middleware
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
REDIS_HOST=localhost
REDIS_PORT=6379
OPENAI_API_KEY=your-api-key
```

## Features in Detail

### Caching Strategy
- Uses Redis for caching quiz answers
- Implements similarity-based cache lookup
- Supports hash-based storage for multiple answer variations
- Configurable cache expiration

### Error Handling
- Domain-specific error types
- Proper HTTP status code mapping
- Detailed error responses
- Middleware-based error handling

### Answer Evaluation
- LLM-based answer evaluation
- Similarity comparison using embeddings
- Keyword matching
- Score breakdown (completeness, relevance, accuracy)

## Contribution & Contact
- PRs and issues are welcome
- Contact: jaeyeong.i.dev@gmail.com