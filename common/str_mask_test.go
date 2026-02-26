package common

import (
	"testing"
)

func TestMaskSensitiveInfo(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string // substrings that MUST appear in output
		excludes []string // substrings that must NOT appear in output
	}{
		{
			name:     "sk-key masking",
			input:    "error: invalid key sk-proj-abcdefghijklmnopqrstuvwxyz123456",
			contains: []string{"sk-***"},
			excludes: []string{"sk-proj-abcdefghijklmnopqrstuvwxyz123456"},
		},
		{
			name:     "sk-key short not masked",
			input:    "prefix sk-short end",
			contains: []string{"sk-short"},
		},
		{
			name:     "Google API key masking",
			input:    "key=AIzaSyABCDEFGHIJKLMNOPQRSTUVWXYZ",
			contains: []string{"AIza***"},
			excludes: []string{"AIzaSyABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		},
		{
			name:     "AWS key masking",
			input:    "aws_key: AKIAIOSFODNN7EXAMPLE",
			contains: []string{"AKIA***"},
			excludes: []string{"AKIAIOSFODNN7EXAMPLE"},
		},
		{
			name:     "Bearer token masking",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc123",
			contains: []string{"Bearer ***"},
			excludes: []string{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
		},
		{
			name:     "Bearer case insensitive",
			input:    "header: bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc123",
			contains: []string{"Bearer ***"},
			excludes: []string{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
		},
		{
			name:     "org-ID masking",
			input:    "org: org-abcdefghijklmnopqrstuvwxyz",
			contains: []string{"org-***"},
			excludes: []string{"org-abcdefghijklmnopqrstuvwxyz"},
		},
		{
			name:     "org-ID short not masked",
			input:    "org-short is fine",
			contains: []string{"org-short"},
		},
		{
			name:     "URL masking still works",
			input:    "error connecting to https://api.openai.com/v1/chat/completions",
			contains: []string{"https://"},
			excludes: []string{"api.openai.com", "/v1/chat/completions"},
		},
		{
			name:     "IP masking still works",
			input:    "connect to 192.168.1.100:8080 failed",
			contains: []string{"***.***.***.***"},
			excludes: []string{"192.168.1.100"},
		},
		{
			name:     "domain masking still works",
			input:    "DNS lookup failed for api.anthropic.com",
			excludes: []string{"api.anthropic.com"},
		},
		{
			name:     "api_key pattern masking",
			input:    `'api_key:AIzaSyAAAaUooTUni8AdaOkSRMda30n_Q4vrV70'`,
			contains: []string{"api_key:***"},
			excludes: []string{"AIzaSyAAAaUooTUni8AdaOkSRMda30n_Q4vrV70"},
		},
		{
			name:     "plain text unchanged",
			input:    "simple error: model not found",
			contains: []string{"simple error: model not found"},
		},
		{
			name:     "empty string",
			input:    "",
			contains: []string{""},
		},
		{
			name:     "multiple sensitive items",
			input:    "key sk-proj-abcdefghijklmnopqrstuvwxyz123456 at org-abcdefghijklmnopqrstuvwxyz on https://api.openai.com/v1",
			contains: []string{"sk-***", "org-***"},
			excludes: []string{"sk-proj-abcdefghijklmnopqrstuvwxyz123456", "org-abcdefghijklmnopqrstuvwxyz", "api.openai.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveInfo(tt.input)
			for _, s := range tt.contains {
				if !containsString(result, s) {
					t.Errorf("expected result to contain %q, got %q", s, result)
				}
			}
			for _, s := range tt.excludes {
				if containsString(result, s) {
					t.Errorf("expected result to NOT contain %q, got %q", s, result)
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
