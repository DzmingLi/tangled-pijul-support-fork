package models

import (
	"encoding/base64"
	"strings"
)

// SecretMask replaces secret values in strings with "***".
type SecretMask struct {
	replacer *strings.Replacer
}

// NewSecretMask creates a mask for the given secret values.
// Also registers base64-encoded variants of each secret.
func NewSecretMask(values []string) *SecretMask {
	var pairs []string

	for _, value := range values {
		if value == "" {
			continue
		}

		pairs = append(pairs, value, "***")

		b64 := base64.StdEncoding.EncodeToString([]byte(value))
		if b64 != value {
			pairs = append(pairs, b64, "***")
		}

		b64NoPad := strings.TrimRight(b64, "=")
		if b64NoPad != b64 && b64NoPad != value {
			pairs = append(pairs, b64NoPad, "***")
		}
	}

	if len(pairs) == 0 {
		return nil
	}

	return &SecretMask{
		replacer: strings.NewReplacer(pairs...),
	}
}

// Mask replaces all registered secret values with "***".
func (m *SecretMask) Mask(input string) string {
	if m == nil || m.replacer == nil {
		return input
	}
	return m.replacer.Replace(input)
}
