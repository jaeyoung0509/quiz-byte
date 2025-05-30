package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/logger"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"go.uber.org/zap"
)

// llmEvaluator implements domain.AnswerEvaluator
type llmEvaluator struct {
	llamaServer string
}

// NewLLMEvaluator creates a new instance of llmEvaluator
func NewLLMEvaluator(llamaServer string) domain.AnswerEvaluator {
	return &llmEvaluator{
		llamaServer: llamaServer,
	}
}

// EvaluateAnswer implements domain.AnswerEvaluator
func (e *llmEvaluator) EvaluateAnswer(questionText string, modelAnswer string, userAnswer string, keywords []string) (*domain.Answer, error) {
	l := logger.Get()
	l.Info("Evaluating answer with LLM",
		zap.String("question", questionText),
		zap.String("model_answer", modelAnswer),
		zap.String("user_answer", userAnswer),
		zap.Strings("keywords", keywords))

	// Create prompt
	prompt := fmt.Sprintf(`You are a quiz answer evaluator. Evaluate the answer and respond with ONLY a JSON object in the following format:
{
    "score": 0.0,
    "explanation": "brief explanation here",
    "keyword_matches": ["matched_keyword1", "matched_keyword2"],
    "completeness": 0.0,
    "relevance": 0.0,
    "accuracy": 0.0
}

Question: %s
Model Answer: %s
User's Answer: %s
Keywords to Check: %s

Rules:
1. All scores must be between 0 and 1 (1 is perfect)
2. Explanation must be under 100 words, focusing on key strengths and areas for improvement
3. keyword_matches should list all keywords from the given set that appear in the user's answer
4. Completeness measures how fully the answer addresses all aspects of the question
5. Relevance measures how well the answer stays on topic
6. Accuracy measures the factual correctness based on the model answers provided`, questionText, modelAnswer, userAnswer, strings.Join(keywords, ", "))

	// Call LLM
	response, err := e.callLLM(prompt)
	if err != nil {
		return nil, domain.NewLLMServiceError(err)
	}

	// Parse response
	var llmResp struct {
		Score          float64  `json:"score"`
		Explanation    string   `json:"explanation"`
		KeywordMatches []string `json:"keyword_matches"`
		Completeness   float64  `json:"completeness"`
		Relevance      float64  `json:"relevance"`
		Accuracy       float64  `json:"accuracy"`
	}

	// Clean and parse response
	responseStr := strings.TrimSpace(response)
	if thinkStart := strings.Index(responseStr, "<think>"); thinkStart != -1 {
		if thinkEnd := strings.Index(responseStr, "</think>"); thinkEnd != -1 {
			responseStr = responseStr[thinkEnd+8:]
		}
	}

	responseStr = strings.TrimPrefix(responseStr, "```json")
	responseStr = strings.TrimPrefix(responseStr, "```")
	responseStr = strings.TrimSuffix(responseStr, "```")
	responseStr = strings.TrimSpace(responseStr)

	if err := json.Unmarshal([]byte(responseStr), &llmResp); err != nil {
		return nil, domain.NewLLMServiceError(err)
	}

	// Create answer with all evaluation fields
	answer := &domain.Answer{
		UserAnswer:     userAnswer,
		Score:          llmResp.Score,
		Explanation:    llmResp.Explanation,
		KeywordMatches: llmResp.KeywordMatches,
		Completeness:   llmResp.Completeness,
		Relevance:      llmResp.Relevance,
		Accuracy:       llmResp.Accuracy,
	}

	return answer, nil
}

// callLLM calls the LLM service
func (e *llmEvaluator) callLLM(prompt string) (string, error) {
	l := logger.Get()

	// HTTP client setup
	httpClient := &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     10 * time.Second,
		},
	}

	// Create LLM client
	llm, err := ollama.New(
		ollama.WithServerURL(e.llamaServer),
		ollama.WithModel("qwen3:0.6b"),
		ollama.WithHTTPClient(httpClient),
	)
	if err != nil {
		l.Error("Failed to create LLM client", zap.Error(err))
		return "", domain.NewLLMServiceError(err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Call LLM
	response, err := llm.Call(ctx, prompt, llms.WithTemperature(0.1))
	if err != nil {
		if err == context.DeadlineExceeded {
			l.Error("LLM request timed out", zap.Error(err))
			return "", domain.NewLLMServiceError(err)
		}
		l.Error("Failed to get response from LLM", zap.Error(err))
		return "", domain.NewLLMServiceError(err)
	}

	return response, nil
}
