package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"quiz-byte/internal/logger" // logger import 확인
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"go.uber.org/zap" // zap import 확인
)

// llmEvaluator implements AnswerEvaluator
type llmEvaluator struct {
	llamaServer string
}

// NewLLMEvaluator creates a new instance of llmEvaluator
func NewLLMEvaluator(llamaServer string) AnswerEvaluator {
	return &llmEvaluator{
		llamaServer: llamaServer,
	}
}

// EvaluateAnswer implements AnswerEvaluator
func (e *llmEvaluator) EvaluateAnswer(questionText string, modelAnswer string, userAnswer string, keywords []string) (*Answer, error) {
	l := logger.Get() // 전역 로거 사용
	l.Info("Evaluating answer with LLM",
		zap.String("question", questionText),
		// modelAnswer, userAnswer는 매우 길 수 있으므로 상세 로깅은 Debug 레벨 또는 필요시에만
		// zap.String("model_answer", modelAnswer),
		// zap.String("user_answer", userAnswer),
		zap.Strings("keywords", keywords))

	// 프롬프트는 이전과 동일하게 유지
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
		l.Error("callLLM failed during LLM evaluation", zap.Error(err), zap.String("prompt_ kısmi", prompt[:min(200, len(prompt))])) // 프롬프트 일부 로깅
		return nil, NewLLMServiceError(fmt.Errorf("callLLM failed: %w", err))
	}

	l.Debug("Raw LLM response received", zap.String("raw_response", rawLLMResponse)) // 원시 응답 로깅 (디버그 레벨)

	cleanedResponseStr := strings.TrimSpace(rawLLMResponse)

	// <think>...</think> 블록 제거 (존재 시)
	if thinkStart := strings.Index(cleanedResponseStr, "<think>"); thinkStart != -1 {
		if thinkEnd := strings.Index(cleanedResponseStr, "</think>"); thinkEnd != -1 && thinkEnd > thinkStart {
			// <think> 블록 앞부분 + 뒷부분
			cleanedResponseStr = cleanedResponseStr[:thinkStart] + cleanedResponseStr[thinkEnd+len("</think>"):]
			cleanedResponseStr = strings.TrimSpace(cleanedResponseStr)
			l.Debug("LLM response after stripping <think> tags", zap.String("cleaned_response", cleanedResponseStr))
		}
	}

	// JSON 객체를 찾기 위해 첫 '{'와 마지막 '}'를 기준으로 추출 시도
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

		// JSON 파싱 시도
		if errUnmarshal := json.Unmarshal([]byte(extractedJSONStr), &llmResp); errUnmarshal != nil {
			l.Error("Failed to unmarshal extracted JSON from LLM response",
				zap.Error(errUnmarshal),
				zap.String("json_string_tried_to_parse", extractedJSONStr),
				zap.String("original_cleaned_llm_response", cleanedResponseStr)) // 디버깅을 위해 원래 정리된 문자열도 로깅
			return nil, NewLLMServiceError(fmt.Errorf("failed to unmarshal JSON from LLM (tried to parse: '%s'): %w", extractedJSONStr, errUnmarshal))
		}

		l.Info("Successfully parsed LLM response", zap.Any("parsed_llm_evaluation", llmResp))

		// Answer 객체 생성
		// QuizID는 LLM 응답에 없으므로, 서비스 계층에서 채워야 함
		answer := &Answer{
			UserAnswer:     userAnswer, // 사용자의 원본 답변
			Score:          llmResp.Score,
			Explanation:    llmResp.Explanation,
			KeywordMatches: llmResp.KeywordMatches,
			Completeness:   llmResp.Completeness,
			Relevance:      llmResp.Relevance,
			Accuracy:       llmResp.Accuracy,
			// QuizID와 AnsweredAt은 서비스 계층에서 Answer 객체 생성 시 설정됨
		}
		return answer, nil

	} else {
		// JSON 구분자 '{' 또는 '}'를 찾지 못한 경우
		l.Error("Could not find valid JSON object delimiters '{' and '}' in LLM response",
			zap.String("cleaned_response_without_json_delimiters", cleanedResponseStr),
			zap.String("original_raw_llm_response", rawLLMResponse))
		return nil, NewLLMServiceError(fmt.Errorf("no JSON object found in LLM response: %s", cleanedResponseStr))
	}
}

// callLLM 함수는 기존과 동일
func (e *llmEvaluator) callLLM(prompt string) (string, error) {
	l := logger.Get()

	httpClient := &http.Client{
		Timeout: 20 * time.Second, // 타임아웃은 환경설정에서 가져오는 것이 더 좋음
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     10 * time.Second,
		},
	}

	llm, err := ollama.New(
		ollama.WithServerURL(e.llamaServer),
		ollama.WithModel("qwen3:0.6b"), // 모델명 확인
		ollama.WithHTTPClient(httpClient),
	)
	if err != nil {
		l.Error("Failed to create LLM client", zap.Error(err))
		return "", fmt.Errorf("failed to create LLM client: %w", err) // NewLLMServiceError 대신 일반 오류 반환 후 상위에서 래핑
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // 타임아웃
	defer cancel()

	response, err := llm.Call(ctx, prompt, llms.WithTemperature(0.1))
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

// min 함수 (helper)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
