package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type OpenAIService struct {
	APIKey string
}

func NewOpenAIService(apiKey string) *OpenAIService {
	return &OpenAIService{APIKey: apiKey}
}

type OpenAIRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

type ResponseFormat struct {
	Type string `json:"type"` // "json_object" | "text"
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

const (
	// maxBatchTokenEstimate is a rough upper bound (chars/4 ≈ tokens).
	// If the JSON is larger than this, we fall back to per-key translation.
	maxBatchChars = 12_000 // ~3 000 tokens — well within gpt-3.5-turbo-0125 16K limit
	maxRetries    = 3
)

// TranslateJSONBatch sends the entire component JSON in a single OpenAI call.
// It uses json_object response_format to guarantee valid JSON output, validates
// that all keys are preserved and all [bracketed] placeholders survive, and
// retries up to maxRetries times on structural or placeholder validation failure.
//
// Falls back to TranslateJSONPerKey if the serialized JSON exceeds maxBatchChars
// (very large components).
func (s *OpenAIService) TranslateJSONBatch(ctx context.Context, data map[string]interface{}, sourceLang, targetLang string) (map[string]interface{}, error) {
	if s.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source JSON: %w", err)
	}

	// Fall back to per-key translation for oversized components
	if len(jsonBytes) > maxBatchChars {
		return s.TranslateJSONPerKey(ctx, data, sourceLang, targetLang)
	}

	prompt := fmt.Sprintf(
		"Translate all string values in the JSON below from %s to %s.\n\n"+
			"STRICT RULES — violating any rule means your output is rejected:\n"+
			"1. Return ONLY valid JSON. No explanation, no markdown, no code fences.\n"+
			"2. Every key in the input must appear in the output, unchanged.\n"+
			"3. Do NOT add any new keys.\n"+
			"4. [bracketed] tokens in values (e.g. [name], [count], [amount]) are placeholders — "+
			"copy them character-for-character into the translated string at the correct position.\n"+
			"5. Translate only string values. Leave numbers, booleans, arrays, and null as-is.\n"+
			"6. If a string value consists entirely of a placeholder (e.g. \"[amount]\"), return it unchanged.\n"+
			"7. URLs (any substring starting with http:// or https://) must be copied verbatim — do NOT translate or alter them.\n"+
			"8. Email addresses (any token matching user@domain) must be copied verbatim — do NOT translate or alter them.\n"+
			"9. If a string value is ONLY a URL or ONLY an email address, return it completely unchanged.\n\n"+
			"Input JSON:\n%s",
		sourceLang, targetLang, string(jsonBytes),
	)

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Bail immediately if context was cancelled between retries
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("cancelled before attempt %d: %w", attempt, err)
		}
		result, err := s.callOpenAIJSON(ctx, prompt)
		if err != nil {
			delay, shouldRetry := openAIRetryDelay(err, attempt)
			if !shouldRetry {
				// Permanent error (bad key, bad request) — no point retrying
				return nil, fmt.Errorf("permanent OpenAI error, aborting: %w", err)
			}
			lastErr = fmt.Errorf("attempt %d: OpenAI call failed: %w", attempt, err)
			if delay > 0 && attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
			}
			continue
		}

		if err := validateTranslatedJSON(data, result); err != nil {
			lastErr = fmt.Errorf("attempt %d: validation failed: %w", attempt, err)
			// Short pause before re-prompting — model may give better output
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(1 * time.Second):
				}
			}
			continue
		}

		return result, nil
	}

	// All retries exhausted — fall back to per-key so we never lose data
	perKeyResult, perKeyErr := s.TranslateJSONPerKey(ctx, data, sourceLang, targetLang)
	if perKeyErr != nil {
		return nil, fmt.Errorf("batch failed after %d retries (%w); per-key fallback also failed: %v", maxRetries, lastErr, perKeyErr)
	}
	return perKeyResult, nil
}

// callOpenAIJSON calls the OpenAI chat completions API with response_format=json_object
// and returns the parsed map. Returns an error if the response is not valid JSON.
// The request is tied to ctx — cancelling ctx (e.g. on SIGTERM) aborts the HTTP call cleanly.
func (s *OpenAIService) callOpenAIJSON(ctx context.Context, prompt string) (map[string]interface{}, error) {
	requestBody := OpenAIRequest{
		Model: "gpt-3.5-turbo-0125", // supports response_format json_object
		Messages: []Message{
			{
				Role: "system",
				Content: "You are a professional JSON translator. " +
					"You always return ONLY valid JSON — never any explanation or markdown. " +
					"Preserve all [bracketed] placeholders from source values exactly.",
			},
			{Role: "user", Content: prompt},
		},
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.APIKey))

	// Hard timeout: belt-and-suspenders in case ctx has no deadline.
	// gpt-3.5-turbo-0125 rarely takes >30s; this prevents goroutine leaks.
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error %d: %s", resp.StatusCode, string(body))
	}

	var openAIResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, err
	}
	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	content := strings.TrimSpace(openAIResp.Choices[0].Message.Content)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("response is not valid JSON: %w (got: %.200s)", err, content)
	}
	return result, nil
}

// validateTranslatedJSON checks:
//  1. All source keys are present in the translation (no key dropped).
//  2. No extra keys were added by the model.
//  3. All [bracketed] placeholders in source string values survive in the translation.
func validateTranslatedJSON(source, translated map[string]interface{}) error {
	for key, srcVal := range source {
		tVal, ok := translated[key]
		if !ok {
			return fmt.Errorf("missing key %q in translated output", key)
		}
		switch sv := srcVal.(type) {
		case string:
			tv, ok := tVal.(string)
			if !ok {
				return fmt.Errorf("key %q: expected string in translation, got %T", key, tVal)
			}
			if err := validatePlaceholders(sv, tv); err != nil {
				return fmt.Errorf("key %q: %w", key, err)
			}
		case map[string]interface{}:
			tvMap, ok := tVal.(map[string]interface{})
			if !ok {
				return fmt.Errorf("key %q: expected object in translation, got %T", key, tVal)
			}
			if err := validateTranslatedJSON(sv, tvMap); err != nil {
				return err
			}
		}
	}
	for key := range translated {
		if _, ok := source[key]; !ok {
			return fmt.Errorf("unexpected extra key %q in translated output", key)
		}
	}
	return nil
}

// validatePlaceholders ensures every [placeholder] from the source string is
// present verbatim in the translated string.
func validatePlaceholders(source, translated string) error {
	srcPlaceholders := ExtractTemplatePlaceholders(source)
	for _, ph := range srcPlaceholders {
		if !strings.Contains(translated, ph) {
			return fmt.Errorf("placeholder %q missing from translated value %q", ph, translated)
		}
	}
	return nil
}

// Translate translates a single string, preserving [bracketed] placeholders.
// Used as the per-key fallback inside TranslateJSONPerKey.
func (s *OpenAIService) Translate(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	if s.APIKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	prompt := fmt.Sprintf(
		"Translate the following text from %s to %s.\n\n"+
			"RULES:\n"+
			"1. Copy any segment that appears inside square brackets in the SOURCE text exactly as-is "+
			"(e.g. [name], [count]). These are placeholders and must not be translated or changed.\n"+
			"2. Translate all other text normally. Do NOT wrap any translated word in square brackets.\n"+
			"3. URLs (substrings starting with http:// or https://) must be copied verbatim — do not translate or alter them.\n"+
			"4. Email addresses (tokens matching user@domain) must be copied verbatim — do not translate or alter them.\n"+
			"5. If the entire text is a URL or email address, return it completely unchanged.\n\n"+
			"Example: \"Hi [name]! Selamat datang di pesta!\" → \"Hi [name]! Welcome to the party!\"\n\n"+
			"Text to translate: %s",
		sourceLang, targetLang, text,
	)

	requestBody := OpenAIRequest{
		Model: "gpt-3.5-turbo-0125",
		Messages: []Message{
			{Role: "system", Content: "You are a professional translator. Preserve only existing [bracketed] placeholders from the source; translate everything else and never add new square brackets."},
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.APIKey))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API error: %s", string(body))
	}

	var openAIResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return "", err
	}
	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no translation returned")
	}

	translated := strings.TrimSpace(openAIResp.Choices[0].Message.Content)
	return PreserveTemplateValues(text, translated), nil
}

// TranslateJSONPerKey translates a JSON map value-by-value (one API call per string).
// Only used as a fallback when batch translation fails or the JSON is too large.
// Keys are always preserved unchanged.
func (s *OpenAIService) TranslateJSONPerKey(ctx context.Context, data map[string]interface{}, sourceLang, targetLang string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for key, value := range data {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("cancelled at key %s: %w", key, err)
		}
		switch v := value.(type) {
		case string:
			translated, err := s.Translate(ctx, v, sourceLang, targetLang)
			if err != nil {
				return nil, fmt.Errorf("error translating key %s: %w", key, err)
			}
			result[key] = translated
		case map[string]interface{}:
			translated, err := s.TranslateJSONPerKey(ctx, v, sourceLang, targetLang)
			if err != nil {
				return nil, err
			}
			result[key] = translated
		default:
			result[key] = v // numbers, booleans, null, arrays — copy as-is
		}
	}
	return result, nil
}

// TranslateJSON is the primary entry point for component-level translation.
// It tries TranslateJSONBatch first (one API call per component) and only
// falls back to TranslateJSONPerKey if batch fails after retries.
func (s *OpenAIService) TranslateJSON(ctx context.Context, data map[string]interface{}, sourceLang, targetLang string) (map[string]interface{}, error) {
	return s.TranslateJSONBatch(ctx, data, sourceLang, targetLang)
}

// GetDefaultOpenAIKey returns the default OpenAI key from environment
func GetDefaultOpenAIKey() string {
	return os.Getenv("OPENAI_API_KEY")
}

// openAIRetryDelay decides whether to retry an OpenAI error, and how long to wait.
//
// Strategy:
//   - 429 (rate limit): exponential backoff 5s → 10s → 20s
//   - 400 / 401 / 403: permanent error — do NOT retry (bad key, malformed request)
//   - 5xx / network error: exponential backoff 2s → 4s → 8s
//   - JSON validation failure (no HTTP code in error): short pause already handled by caller
func openAIRetryDelay(err error, attempt int) (delay time.Duration, shouldRetry bool) {
	if err == nil {
		return 0, false
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "OpenAI API error 429"):
		// Rate-limited: back off aggressively
		return time.Duration(5*(1<<uint(attempt-1))) * time.Second, true
	case strings.Contains(msg, "OpenAI API error 400"),
		strings.Contains(msg, "OpenAI API error 401"),
		strings.Contains(msg, "OpenAI API error 403"):
		// Permanent: bad request, invalid key, forbidden
		return 0, false
	default:
		// Network error, 5xx, timeout — retry with backoff
		return time.Duration(2*(1<<uint(attempt-1))) * time.Second, true
	}
}
