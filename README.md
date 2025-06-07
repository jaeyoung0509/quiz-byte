<img src="logo.png" alt="Quiz Byte Logo" width="200">

# Quiz Byte Backend

## Overview
Quiz Byte is a backend system that provides a variety of quizzes in computer science and IT. It is built with Go, Oracle DB, and follows Clean Architecture principles with Domain-Driven Design. The system supports user authentication via Google OAuth, AI-powered quiz generation and evaluation, and provides personalized quiz recommendations.

## Features
- User Authentication via Google OAuth 2.0
- Category and subcategory-based quiz delivery
- AI-powered quiz generation using Gemini LLM
- LLM-based auto-grading with detailed feedback (completeness, relevance, accuracy)
- Similarity-based answer evaluation using embeddings (OpenAI/Ollama)
- User progress tracking and quiz attempt history
- Personalized quiz recommendations
- RESTful API with comprehensive error handling
- Caching support with Redis (using Port & Adapter pattern)
- Integration tests and DB migration support
- Domain-driven error handling and validation

## New Features (Recent Updates)
- **Transaction Management**: Consistent transaction handling across all service layers for data integrity
- **Database Robustness**: Enhanced NULL handling for Oracle database compatibility
- **Service Layer Improvements**: Unified service constructor patterns with transaction support
- **User Management**: Complete user authentication and profile management
- **Quiz Attempt Tracking**: Store and retrieve user quiz attempts with detailed evaluation
- **AI-Powered Evaluation**: Advanced answer evaluation using LLM with multiple scoring metrics
- **Embedding-Based Similarity**: Smart answer comparison using vector embeddings
- **Quiz Generation**: Automated quiz generation from text content using AI
- **Batch Processing**: Bulk quiz creation and management capabilities
- **Recommendation System**: Personalized quiz suggestions based on user performance

## Architecture
The project follows Clean Architecture principles with Domain-Driven Design:
- **Domain Layer**: Core business logic, entities, and interfaces
- **Service Layer**: Application use cases and business logic orchestration with transaction management
- **Repository Layer**: Data access with adapters and database abstraction
- **Handler Layer**: HTTP request handling and API endpoints
- **Infrastructure Layer**: External services (Redis, OAuth, LLM services)
- **Adapter Layer**: Integration with external services (embeddings, quiz generation)
- **Transaction Layer**: Consistent transaction boundaries across all operations

## Project Structure
```
cmd/                          # Entry points
  api/                        # Main API server
  batch_add_questions/        # Batch processing tool
  migrate/                    # Database migration tool
internal/           
  adapter/                    # Infrastructure adapters
    embedding/                # Embedding services (OpenAI, Ollama)
    quizgen/                  # Quiz generation services (Gemini)
  cache/                      # Cache abstraction and Redis implementation
  domain/                     # Domain models, entities, and interfaces
    quiz.go                   # Quiz domain model with difficulty constants
    user.go                   # User domain model
    evaluator.go              # Answer evaluation interfaces
    embedding.go              # Embedding service interfaces
    quizgen.go               # Quiz generation interfaces
  repository/                 # Data access layer
    models/                   # Database models
    quiz_database_adapter.go  # Quiz data operations
    user_repository.go        # User data operations
  service/                    # Application services
    quiz.go                   # Quiz business logic
    user_service.go           # User management
    auth_service.go           # Authentication logic
    answer_cache.go           # Smart answer caching
    batch_service.go          # Batch processing
  handler/                    # HTTP handlers
    quiz.go                   # Quiz API endpoints
    user_handler.go           # User API endpoints
    auth_handler.go           # Authentication endpoints
  dto/                        # API DTOs and request/response models
  middleware/                 # HTTP middleware (auth, error handling)
  logger/                     # Structured logging
  database/                   # DB connection and migration
configs/                      # Configuration files
tests/integration/            # Integration tests
```

## Technology Stack
- **Backend**: Go 1.20+
- **Database**: Oracle Database
- **Cache**: Redis
- **Authentication**: Google OAuth 2.0
- **AI Services**: 
  - Google Gemini (Quiz Generation)
  - OpenAI (Embeddings)
  - Ollama (Local Embeddings)
- **Testing**: Testify, Integration Tests
- **Infrastructure**: Docker, Docker Compose

## Getting Started

### Prerequisites
- Go 1.20+
- Oracle Database (local/remote)
- Redis Server
- Docker & Docker Compose (recommended)
- API Keys for AI services (optional for basic functionality)

### Installation
1. Clone the repository
```bash
git clone <repository-url>
cd quize-byte
```

2. Install dependencies
```bash
go mod tidy
```

3. Set up configuration
```bash
cp config.yaml.example config.yaml
# Edit config.yaml with your settings
```

4. Start infrastructure services
```bash
docker-compose up -d  # Starts Oracle DB and Redis
```

5. Run database migrations
```bash
go run cmd/migrate/main.go
```

6. Start the API server
```bash
go run cmd/api/main.go
```

### Configuration
Key environment variables and config settings:

```yaml
database:
  user: system
  password: oracle
  host: localhost
  port: 1521
  service_name: FREE

redis:
  address: localhost:6379
  db: 0

auth:
  google:
    client_id: your-google-client-id
    client_secret: your-google-client-secret
    redirect_url: http://localhost:8080/auth/google/callback

llm:
  gemini:
    api_key: your-gemini-api-key
    model: gemini-pro

embedding:
  source: openai  # or "ollama"
  openai:
    api_key: your-openai-api-key
    model: text-embedding-ada-002
  ollama:
    server_url: http://localhost:11434
    model: nomic-embed-text
```

## Command-Line Tools

This section describes various command-line tools available in the project.

### Initial Data Seeder

This tool is used to populate the database with an initial set of categories, sub-categories, and quizzes. This is useful for setting up a new environment or for testing purposes.

**Location of the seeder command:**
`cmd/seed_initial_data/main.go`

**Location of the seed data file:**
`configs/seed_data/initial_english_quizzes.json`

**How to run the seeder:**

1.  Ensure your database is running and accessible as per the `config.yaml` settings.
2.  Make sure database migrations have been applied:
    ```bash
    go run cmd/migrate/main.go
    ```
3.  Run the seeder command from the root of the project:
    ```bash
    go run cmd/seed_initial_data/main.go
    ```

The command will log its progress to the console. It is designed to be idempotent for categories and sub-categories: it will not create duplicate categories (by name) or duplicate sub-categories (by name, within the same parent category). Quizzes are always added as new items under their respective sub-categories each time the seeder processes that sub-category; it does not check for duplicate quizzes by content.

## API Endpoints

### Authentication
- `POST /auth/google/login` - Initiate Google OAuth login
- `POST /auth/google/callback` - Handle OAuth callback
- `POST /auth/refresh` - Refresh JWT token

### Quiz Management
- `GET /quiz/categories` - Get all quiz categories
- `GET /quiz/random/:category` - Get random quiz by category
- `POST /quiz/check` - Submit and evaluate quiz answer
- `POST /quiz/bulk` - Get multiple quizzes with criteria

### User Management
- `GET /users/me` - Get user profile
- `GET /users/me/attempts` - Get user's quiz attempts
- `GET /users/me/incorrect-answers` - Get incorrect answers for review
- `GET /users/me/recommendations` - Get personalized quiz recommendations

## Advanced Features

### AI-Powered Answer Evaluation
The system uses LLM-based evaluation with multiple scoring metrics:
- **Score**: Overall correctness (0.0-1.0)
- **Completeness**: How complete the answer is
- **Relevance**: How relevant the answer is to the question
- **Accuracy**: Technical accuracy of the answer
- **Keyword Matching**: Important keywords found in the answer

### Smart Caching
- Embedding-based similarity search for cached answers
- Reduces redundant LLM calls for similar answers
- Configurable similarity thresholds
- Hash-based storage for multiple answer variations

### Batch Processing
- Bulk quiz generation from text content
- Automated categorization and difficulty assignment
- Progress tracking and error handling
- Suitable for content migration and bulk updates

### Quiz Recommendation System
- Analyzes user performance patterns
- Recommends quizzes based on knowledge gaps
- Considers subcategory preferences
- Adaptive difficulty progression

## Testing

### Unit Tests
```bash
go test ./internal/...
```

### Integration Tests
```bash
go test ./tests/integration/...
```

### Test Coverage
```bash
go test -cover ./internal/...
```

## Development Guidelines

### Domain-Driven Design
- Domain entities are in `internal/domain/`
- Business logic is encapsulated in services
- Repository pattern for data access
- Clean separation of concerns

### Error Handling
- Domain-specific error types
- Proper HTTP status code mapping
- Structured error responses
- Context-aware error messages

### Code Quality
- Interface-based design for testability
- Dependency injection pattern
- Comprehensive test coverage
- Proper logging and monitoring

## Deployment
The application is designed for cloud deployment with:
- Containerized services
- Environment-based configuration
- Health check endpoints
- Graceful shutdown handling

## Contributing
1. Fork the repository
2. Create a feature branch
3. Implement changes with tests
4. Submit a pull request

## License
[Add your license information]
- Contact: jaeyeong.i.dev@gmail.com