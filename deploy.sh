#!/bin/bash

# OCI Deployment Script for Quiz Application

set -e

echo "🚀 Starting OCI deployment for Quiz Application..."

# Check if .env file exists
if [ ! -f .env ]; then
    echo "❌ .env file not found. Please copy .env.example to .env and fill in your values."
    exit 1
fi

# Load environment variables
source .env

# Check if models directory exists and create it if not
if [ ! -d "models" ]; then
    echo "📁 Creating models directory..."
    mkdir -p models
fi

# Download Qwen model if it doesn't exist
MODEL_FILE="models/qwen2.5-0.5b-instruct-q4_k_m.gguf"
if [ ! -f "$MODEL_FILE" ]; then
    echo "📥 Downloading Qwen 2.5 0.5B model..."
    echo "Please download the model manually from Hugging Face:"
    echo "https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF"
    echo "Save it as: $MODEL_FILE"
    echo ""
    echo "Or use wget/curl:"
    echo "wget -O $MODEL_FILE https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF/resolve/main/qwen2.5-0.5b-instruct-q4_k_m.gguf"
    exit 1
fi

# Build and start services
echo "🔨 Building and starting services..."
docker-compose down --remove-orphans
docker-compose build --no-cache
docker-compose up -d

echo "✅ Services started successfully!"

# Show status
echo ""
echo "📊 Service Status:"
docker-compose ps

echo ""
echo "🔍 Service URLs:"
echo "  • Quiz API: http://localhost:8080"
echo "  • Llama.cpp Server: http://localhost:8081"
echo "  • Redis: localhost:6379"

echo ""
echo "📝 To check logs:"
echo "  docker-compose logs -f quiz-api"
echo "  docker-compose logs -f llama-qwen"
echo "  docker-compose logs -f redis"

echo ""
echo "🧪 To test the API:"
echo "  curl http://localhost:8080/health"

echo ""
echo "🚀 Deployment completed!"
