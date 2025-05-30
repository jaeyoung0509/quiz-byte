.PHONY: test test-integration

test:
	go test -v ./...

test-integration:
	@echo "Downloading LLM model..."
	@mkdir -p models
	@if [ ! -f models/gemma-2b-it-q4_k_m.gguf ]; then \
		curl -L https://huggingface.co/TheBloke/gemma-2b-it-GGUF/resolve/main/gemma-2b-it-q4_k_m.gguf -o models/gemma-2b-it-q4_k_m.gguf; \
	fi
	@echo "Starting test services..."
	@docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for services to start..."
	@sleep 5
	@echo "Running integration tests..."
	# @ENV=test \
	# DB_HOST=localhost \
	# DB_PORT=1522 \
	# DB_USER=system \
	# DB_PASSWORD=oracle \
	# DB_SERVICE=XE \
	# LLM_SOLVER_ENDPOINT=http://localhost:8082 \
	# LLM_SOLVER_API_KEY=test_key \
	# LLM_GENERATOR_ENDPOINT=http://localhost:8083 \
	# LLM_GENERATOR_API_KEY=test_key \
	go test -v ./tests/integration/...
	@echo "Cleaning up..."
	# @docker-compose -f docker-compose.test.yml down 