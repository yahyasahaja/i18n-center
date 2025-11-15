package i18ncenter

import (
	"fmt"
	"strings"
)

// SyncTranslator provides synchronous translation functions (no API calls)
// Use this when you already have the translation data loaded
type SyncTranslator struct {
	data TranslationData
}

// NewSyncTranslator creates a synchronous translator from preloaded data
func NewSyncTranslator(data TranslationData) *SyncTranslator {
	return &SyncTranslator{
		data: data,
	}
}

// T translates a path to a string value (synchronous, no API calls)
func (t *SyncTranslator) T(path string, defaultValue ...string) string {
	value := getNestedValue(t.data, path)
	if value == nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return path
	}

	return fmt.Sprintf("%v", value)
}

// Tf translates with template variables (synchronous)
func (t *SyncTranslator) Tf(path string, variables map[string]interface{}, defaultValue ...string) string {
	text := t.T(path, defaultValue...)

	// Replace template variables
	for key, val := range variables {
		text = strings.ReplaceAll(text, fmt.Sprintf("{%s}", key), fmt.Sprintf("%v", val))
		text = strings.ReplaceAll(text, fmt.Sprintf("[%s]", key), fmt.Sprintf("%v", val))
	}

	return text
}

// GetRaw returns the raw translation data
func (t *SyncTranslator) GetRaw() TranslationData {
	return t.data
}

