package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"quiz-byte/internal/logger" // Verify logger import
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"go.uber.org/zap" // Verify zap import
)

// llmEvaluator implements AnswerEvaluator
type llmEvaluator struct {
	llmClient *ollama.LLM
}

// NewLLMEvaluator creates a new instance of llmEvaluator
func NewLLMEvaluator(llm *ollama.LLM) AnswerEvaluator {
	return &llmEvaluator{
		llmClient: llm,
	}
}

// EvaluateAnswer implements AnswerEvaluator
func (e *llmEvaluator) EvaluateAnswer(questionText string, modelAnswer string, userAnswer string, keywords []string) (*Answer, error) {
	l := logger.Get() // Use global logger
	l.Info("Evaluating answer with LLM",
		zap.String("question", questionText),
		// modelAnswer, userAnswer can be very long, so detailed logging is at Debug level or only when necessary
		// zap.String("model_answer", modelAnswer),
		// zap.String("user_answer", userAnswer),
		zap.Strings("keywords", keywords))

	// Prompt remains the same as before
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

	rawLLMResponse, err := e.callLLM(prompt)
	if err != nil {
		l.Error("callLLM failed during LLM evaluation", zap.Error(err), zap.String("prompt_part", prompt[:min(200, len(prompt))])) // Log part of the prompt
		return nil, NewLLMServiceError(fmt.Errorf("callLLM failed: %w", err))
	}

	l.Debug("Raw LLM response received", zap.String("raw_response", rawLLMResponse)) // Log raw response (debug level)

	cleanedResponseStr := strings.TrimSpace(rawLLMResponse)

	// Remove <think>...</think> block (if present)
	if thinkStart := strings.Index(cleanedResponseStr, "<think>"); thinkStart != -1 {
		if thinkEnd := strings.Index(cleanedResponseStr, "</think>"); thinkEnd != -1 && thinkEnd > thinkStart {
			// Part before <think> block + part after
			cleanedResponseStr = cleanedResponseStr[:thinkStart] + cleanedResponseStr[thinkEnd+len("</think>"):]
			cleanedResponseStr = strings.TrimSpace(cleanedResponseStr)
			l.Debug("LLM response after stripping <think> tags", zap.String("cleaned_response", cleanedResponseStr))
		}
	}

	// Attempt to extract based on the first '{' and last '}' to find the JSON object
	jsonStart := strings.Index(cleanedResponseStr, "{")
	jsonEnd := strings.LastIndex(cleanedResponseStr, "}")

	if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
		extractedJSONStr := cleanedResponseStr[jsonStart : jsonEnd+1]
		l.Info("Attempting to parse extracted JSON string from LLM", zap.String("extracted_json", extractedJSONStr))

		var llmResp struct {
			Score          float64  `json:"score"`
			Explanation    string   `json:"explanation"`
			KeywordMatches []string `json:"keyword_matches"`
			Completeness   float64  `json:"completeness"`
			Relevance      float64  `json:"relevance"`
			Accuracy       float64  `json:"accuracy"`
		}

		// Attempt JSON parsing
		if errUnmarshal := json.Unmarshal([]byte(extractedJSONStr), &llmResp); errUnmarshal != nil {
			l.Error("Failed to unmarshal extracted JSON from LLM response",
				zap.Error(errUnmarshal),
				zap.String("json_string_tried_to_parse", extractedJSONStr),
				zap.String("original_cleaned_llm_response", cleanedResponseStr)) // Log the original cleaned string for debugging
			return nil, NewLLMServiceError(fmt.Errorf("failed to unmarshal JSON from LLM (tried to parse: '%s'): %w", extractedJSONStr, errUnmarshal))
		}

		l.Info("Successfully parsed LLM response", zap.Any("parsed_llm_evaluation", llmResp))

		// Create Answer object
		// QuizID is not in the LLM response, so it must be filled in the service layer
		answer := &Answer{
			UserAnswer:     userAnswer, // User's original answer
			Score:          llmResp.Score,
			Explanation:    llmResp.Explanation,
			KeywordMatches: llmResp.KeywordMatches,
			Completeness:   llmResp.Completeness,
			Relevance:      llmResp.Relevance,
			Accuracy:       llmResp.Accuracy,
			// QuizID and AnsweredAt are set when creating the Answer object in the service layer
		}
		return answer, nil

	} else {
		// If JSON delimiters '{' or '}' are not found
		l.Error("Could not find valid JSON object delimiters '{' and '}' in LLM response",
			zap.String("cleaned_response_without_json_delimiters", cleanedResponseStr),
			zap.String("original_raw_llm_response", rawLLMResponse))
		return nil, NewLLMServiceError(fmt.Errorf("no JSON object found in LLM response: %s", cleanedResponseStr))
	}
}

// callLLM uses the llmClient from the struct
func (e *llmEvaluator) callLLM(prompt string) (string, error) {
	l := logger.Get()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // Timeout
	defer cancel()

	response, err := e.llmClient.Call(ctx, prompt, llms.WithTemperature(0.1))
	if err != nil {
		if err == context.DeadlineExceeded {
			l.Error("LLM request timed out", zap.Error(err))
			return "", fmt.Errorf("LLM request timed out: %w", err)
		}
		l.Error("Failed to get response from LLM", zap.Error(err))
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	return response, nil
}

// min function (helper)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
