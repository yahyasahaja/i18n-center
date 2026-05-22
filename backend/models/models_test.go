package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// JSONB and StringArray are still real types in this package — exercise their
// sql.Scanner / driver.Valuer interfaces. The BeforeCreate / TableName tests
// were tied to the now-removed GORM hooks; UUID generation is the repository
// layer's responsibility under Commit I, so those tests are gone.

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

func TestStringArray_ValueAndScan(t *testing.T) {
	arr := StringArray{"en", "id"}
	v, err := arr.Value()
	require.NoError(t, err)
	assert.Equal(t, `{"en","id"}`, v)

	var scanned StringArray
	require.NoError(t, scanned.Scan([]byte(`{"en","id"}`)))
	assert.Equal(t, StringArray{"en", "id"}, scanned)

	var nilScan StringArray
	require.NoError(t, nilScan.Scan(nil))
	assert.Nil(t, nilScan)

	err = scanned.Scan(42)
	assert.Error(t, err)
}
