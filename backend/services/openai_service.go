package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type OpenAIService struct {
	APIKey string
}

func NewOpenAIService(apiKey string) *OpenAIService {
	return &OpenAIService{APIKey: apiKey}
}

type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
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

// Translate translates text to target language, preserving template values
func (s *OpenAIService) Translate(text, sourceLang, targetLang string) (string, error) {
	if s.APIKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	// Create prompt
	prompt := fmt.Sprintf(
		"Translate the following text from %s to %s. "+
			"IMPORTANT: Do NOT translate anything inside square brackets []. "+
			"Preserve all template values exactly as they are. "+
			"Only translate the text outside the brackets.\n\nText to translate: %s",
		sourceLang, targetLang, text,
	)

	requestBody := OpenAIRequest{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{Role: "system", Content: "You are a professional translator. Always preserve template values in square brackets."},
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.APIKey))

	client := &http.Client{}
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

	// Ensure template values are preserved
	translated = PreserveTemplateValues(text, translated)

	return translated, nil
}

// TranslateJSON translates a JSON structure recursively
func (s *OpenAIService) TranslateJSON(data map[string]interface{}, sourceLang, targetLang string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range data {
		switch v := value.(type) {
		case string:
			translated, err := s.Translate(v, sourceLang, targetLang)
			if err != nil {
				return nil, fmt.Errorf("error translating key %s: %w", key, err)
			}
			result[key] = translated
		case map[string]interface{}:
			translated, err := s.TranslateJSON(v, sourceLang, targetLang)
			if err != nil {
				return nil, err
			}
			result[key] = translated
		default:
			result[key] = v
		}
	}

	return result, nil
}

// GetDefaultOpenAIKey returns the default OpenAI key from environment
func GetDefaultOpenAIKey() string {
	return os.Getenv("OPENAI_API_KEY")
}

