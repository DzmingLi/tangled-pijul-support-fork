package extension_test

import (
	"bytes"
	"testing"

	"tangled.org/core/appview/pages/markup"
)

func TestTangledLinkExtension_Rendering(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		expected string
	}{
		{
			name:     "renders simple commit link from commonmark autolink",
			markdown: "This is a commit: <https://tangled.org/alice.tngl.sh/cool-repo/commit/cde4705021a07e3cb11322fb9ef78a6c786b41c0>",
			expected: `<p>This is a commit: <a href="https://tangled.org/alice.tngl.sh/cool-repo/commit/cde4705021a07e3cb11322fb9ef78a6c786b41c0"><code>cde47050</code></a></p>`,
		},
		{
			name:     "renders simple commit link from gfm autolink",
			markdown: "This is a commit: https://tangled.org/alice.tngl.sh/cool-repo/commit/cde4705021a07e3cb11322fb9ef78a6c786b41c0",
			expected: `<p>This is a commit: <a href="https://tangled.org/alice.tngl.sh/cool-repo/commit/cde4705021a07e3cb11322fb9ef78a6c786b41c0"><code>cde47050</code></a></p>`,
		},
		{
			name:     "skip non-autolink links",
			markdown: "This is a commit: [a commit](https://tangled.org/alice.tngl.sh/cool-repo/commit/cde4705021a07e3cb11322fb9ef78a6c786b41c0)",
			expected: `<p>This is a commit: <a href="https://tangled.org/alice.tngl.sh/cool-repo/commit/cde4705021a07e3cb11322fb9ef78a6c786b41c0">a commit</a></p>`,
		},
		{
			name:     "skip non-autolink links with content same as dest",
			markdown: "This is a commit: [https://tangled.org/alice.tngl.sh/cool-repo/commit/cde4705021a07e3cb11322fb9ef78a6c786b41c0](https://tangled.org/alice.tngl.sh/cool-repo/commit/cde4705021a07e3cb11322fb9ef78a6c786b41c0)",
			expected: `<p>This is a commit: <a href="https://tangled.org/alice.tngl.sh/cool-repo/commit/cde4705021a07e3cb11322fb9ef78a6c786b41c0">https://tangled.org/alice.tngl.sh/cool-repo/commit/cde4705021a07e3cb11322fb9ef78a6c786b41c0</a></p>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := markup.NewMarkdown()

			var buf bytes.Buffer
			if err := md.Convert([]byte(tt.markdown), &buf); err != nil {
				t.Fatalf("failed to convert markdown: %v", err)
			}

			result := buf.String()
			if result != tt.expected+"\n" {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, result)
			}
		})
	}
}
