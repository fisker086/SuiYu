package agent

import (
	"testing"
)

func TestSanitizeJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "boolean true removed",
			input:    true,
			expected: nil,
		},
		{
			name:     "boolean false removed",
			input:    false,
			expected: nil,
		},
		{
			name:     "nested boolean removed",
			input:    map[string]any{"type": "object", "items": true},
			expected: map[string]any{"type": "object"},
		},
		{
			name: "nested map with boolean",
			input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":   map[string]any{"type": "string"},
					"active": true,
				},
			},
			expected: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
		{
			name: "boolean default value removed",
			input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"enabled": map[string]any{
						"type":    "boolean",
						"default": true,
					},
				},
			},
			expected: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"enabled": map[string]any{
						"type": "boolean",
					},
				},
			},
		},
		{
			name: "array with boolean removed",
			input: map[string]any{
				"type":  "array",
				"items": []any{true, map[string]any{"type": "string"}},
			},
			expected: map[string]any{
				"type":  "array",
				"items": []any{map[string]any{"type": "string"}},
			},
		},
		{
			name: "anyOf with boolean removed from array",
			input: map[string]any{
				"anyOf": []any{
					true,
					false,
					map[string]any{"type": "string"},
				},
			},
			expected: map[string]any{
				"anyOf": []any{map[string]any{"type": "string"}},
			},
		},
		{
			name:     "string preserved",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "number preserved",
			input:    42,
			expected: 42,
		},
		{
			name: "complex valid schema preserved",
			input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "User name",
					},
					"age": map[string]any{
						"type":    "integer",
						"default": 0,
					},
					"tags": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
				"required": []any{"name"},
			},
			expected: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "User name",
					},
					"age": map[string]any{
						"type":    "integer",
						"default": 0,
					},
					"tags": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
				"required": []any{"name"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeJSONSchema(tt.input)
			if !deepEqual(result, tt.expected) {
				t.Errorf("sanitizeJSONSchema() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func deepEqual(a, b any) bool {
	switch av := a.(type) {
	case nil:
		return b == nil
	case bool, string, int, int64, float64:
		return av == b
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			return false
		}
		if len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			if !deepEqual(v, bv[k]) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok {
			return false
		}
		if len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !deepEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
