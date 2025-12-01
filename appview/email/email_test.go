package email

import (
	"testing"
)

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		// Valid emails using RFC 2606 reserved domains
		{"standard email", "user@example.com", true},
		{"single char local", "a@example.com", true},
		{"dot in middle", "first.last@example.com", true},
		{"multiple dots", "a.b.c@example.com", true},
		{"plus tag", "user+tag@example.com", true},
		{"numbers", "user123@example.com", true},
		{"example.org", "user@example.org", true},
		{"example.net", "user@example.net", true},

		// Invalid format - rejected by mail.ParseAddress
		{"empty string", "", false},
		{"no at sign", "userexample.com", false},
		{"no domain", "user@", false},
		{"no local part", "@example.com", false},
		{"double at", "user@@example.com", false},
		{"just at sign", "@", false},
		{"leading dot", ".user@example.com", false},
		{"trailing dot", "user.@example.com", false},
		{"consecutive dots", "user..name@example.com", false},

		// Whitespace - rejected before parsing
		{"space in local", "user @example.com", false},
		{"space in domain", "user@ example.com", false},
		{"tab", "user\t@example.com", false},
		{"newline", "user\n@example.com", false},

		// MX lookup - using RFC 2606 reserved TLDs (guaranteed no MX)
		{"invalid TLD", "user@example.invalid", false},
		{"test TLD", "user@mail.test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidEmail(tt.email)
			if got != tt.want {
				t.Errorf("IsValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}
