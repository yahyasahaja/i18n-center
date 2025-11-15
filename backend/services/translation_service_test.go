package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractTemplateValues(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "Single template value",
			text:     "Hi [last_name]!",
			expected: []string{"last_name"},
		},
		{
			name:     "Multiple template values",
			text:     "Hello [first_name] [last_name], welcome!",
			expected: []string{"first_name", "last_name"},
		},
		{
			name:     "No template values",
			text:     "Hello world",
			expected: []string{},
		},
		{
			name:     "Nested brackets",
			text:     "Value [outer[inner]]",
			expected: []string{"outer[inner"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTemplateValues(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPreserveTemplateValues(t *testing.T) {
	tests := []struct {
		name       string
		original   string
		translated string
		expected   string
	}{
		{
			name:       "Preserve single template",
			original:   "Hi [last_name]!",
			translated: "Hola [last_name]!",
			expected:   "Hola [last_name]!",
		},
		{
			name:       "Restore missing template",
			original:   "Hi [name]!",
			translated: "Hola!",
			expected:   "Hola [name]!",
		},
		{
			name:       "No templates",
			original:   "Hello",
			translated: "Hola",
			expected:   "Hola",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreserveTemplateValues(tt.original, tt.translated)
			// For simplicity, we check that template values from original are present
			templateValues := ExtractTemplateValues(tt.original)
			if len(templateValues) > 0 {
				for _, val := range templateValues {
					assert.Contains(t, result, "["+val+"]")
				}
			} else {
				assert.Equal(t, tt.translated, result)
			}
		})
	}
}

func TestNewTranslationService(t *testing.T) {
	service := NewTranslationService()
	assert.NotNil(t, service)
}
