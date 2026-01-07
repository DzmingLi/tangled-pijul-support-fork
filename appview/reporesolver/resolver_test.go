package reporesolver

import "testing"

func TestExtractCurrentDir(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/@user/repo/blob/main/docs/README.md", "docs"},
		{"/@user/repo/blob/main/README.md", "."},
		{"/@user/repo/tree/main/docs", "docs"},
		{"/@user/repo/tree/main/docs/", "docs"},
		{"/@user/repo/tree/main", "."},
	}

	for _, tt := range tests {
		if got := extractCurrentDir(tt.path); got != tt.want {
			t.Errorf("extractCurrentDir(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
