package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestJSONB_Value(t *testing.T) {
	tests := []struct {
		name    string
		jsonb   JSONB
		wantErr bool
	}{
		{
			name:    "Valid JSONB",
			jsonb:   JSONB{"key": "value"},
			wantErr: false,
		},
		{
			name:    "Nil JSONB",
			jsonb:   nil,
			wantErr: false,
		},
		{
			name:    "Empty JSONB",
			jsonb:   JSONB{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.jsonb.Value()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.jsonb == nil {
					assert.Nil(t, value)
				} else {
					assert.NotNil(t, value)
				}
			}
		})
	}
}

func TestJSONB_Scan(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		wantErr bool
	}{
		{
			name:    "Valid JSON bytes",
			value:   []byte(`{"key":"value"}`),
			wantErr: false,
		},
		{
			name:    "Nil value",
			value:   nil,
			wantErr: false,
		},
		{
			name:    "Invalid type",
			value:   "not bytes",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var jsonb JSONB
			err := jsonb.Scan(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUser_BeforeCreate(t *testing.T) {
	user := &User{}
	err := user.BeforeCreate(nil)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID)
}

func TestApplication_BeforeCreate(t *testing.T) {
	app := &Application{}
	err := app.BeforeCreate(nil)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, app.ID)
}

func TestComponent_BeforeCreate(t *testing.T) {
	component := &Component{}
	err := component.BeforeCreate(nil)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, component.ID)
}

func TestTranslationVersion_BeforeCreate(t *testing.T) {
	tv := &TranslationVersion{}
	err := tv.BeforeCreate(nil)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, tv.ID)
}
