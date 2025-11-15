package i18ncenter

import (
	"fmt"
	"strings"
)

// Translator provides translation functions for a specific component
type Translator struct {
	client         *Client
	applicationCode string
	componentCode  string
	locale         string
	stage          DeploymentStage
	cachedData     TranslationData
}

// NewTranslator creates a new translator for a specific component
// applicationCode is required to differentiate components with the same code in different applications
func NewTranslator(client *Client, applicationCode string, componentCode string, locale string, stage DeploymentStage) *Translator {
	if locale == "" {
		locale = client.config.DefaultLocale
	}
	if stage == "" {
		stage = client.config.DefaultStage
	}

	return &Translator{
		client:          client,
		applicationCode: applicationCode,
		componentCode:   componentCode,
		locale:          locale,
		stage:           stage,
	}
}

// T translates a path to a string value
// Path uses dot notation: "form.name.label" -> translation["form"]["name"]["label"]
// Returns the translated string or the path itself if not found
func (t *Translator) T(path string, defaultValue ...string) (string, error) {
	data, err := t.getData()
	if err != nil {
		return "", err
	}

	value := getNestedValue(data, path)
	if value == nil {
		if len(defaultValue) > 0 {
			return defaultValue[0], nil
		}
		return path, nil
	}

	return fmt.Sprintf("%v", value), nil
}

// Tf translates with template variables
// Supports {variable} and [variable] syntax
func (t *Translator) Tf(path string, variables map[string]interface{}, defaultValue ...string) (string, error) {
	text, err := t.T(path, defaultValue...)
	if err != nil {
		return "", err
	}

	// Replace template variables
	for key, val := range variables {
		text = strings.ReplaceAll(text, fmt.Sprintf("{%s}", key), fmt.Sprintf("%v", val))
		text = strings.ReplaceAll(text, fmt.Sprintf("[%s]", key), fmt.Sprintf("%v", val))
	}

	return text, nil
}

// GetRaw returns the raw translation data for the component
func (t *Translator) GetRaw() (TranslationData, error) {
	return t.getData()
}

// Preload preloads the translation data
func (t *Translator) Preload() error {
	data, err := t.client.GetTranslation(t.applicationCode, t.componentCode, t.locale, t.stage)
	if err != nil {
		return err
	}
	t.cachedData = data
	return nil
}

// getData gets the translation data (from cache or API)
func (t *Translator) getData() (TranslationData, error) {
	if t.cachedData != nil {
		return t.cachedData, nil
	}

	data, err := t.client.GetTranslation(t.applicationCode, t.componentCode, t.locale, t.stage)
	if err != nil {
		return nil, err
	}

	t.cachedData = data
	return data, nil
}

// getNestedValue gets a nested value from a map using dot notation
func getNestedValue(data map[string]interface{}, path string) interface{} {
	keys := strings.Split(path, ".")
	current := interface{}(data)

	for _, key := range keys {
		if currentMap, ok := current.(map[string]interface{}); ok {
			if val, exists := currentMap[key]; exists {
				current = val
			} else {
				return nil
			}
		} else {
			return nil
		}
	}

	// If it's a string, return it
	if str, ok := current.(string); ok {
		return str
	}

	// If it's an object, try to find common fields
	if obj, ok := current.(map[string]interface{}); ok {
		if text, ok := obj["text"].(string); ok {
			return text
		}
		if value, ok := obj["value"].(string); ok {
			return value
		}
		if label, ok := obj["label"].(string); ok {
			return label
		}
		// Return JSON string representation
		return fmt.Sprintf("%v", obj)
	}

	return current
}

