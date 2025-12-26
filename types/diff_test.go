package types

import "testing"

func TestDiffId(t *testing.T) {
	tests := []struct {
		name     string
		diff     Diff
		expected string
	}{
		{
			name: "regular file uses new name",
			diff: Diff{
				Name: struct {
					Old string `json:"old"`
					New string `json:"new"`
				}{Old: "", New: "src/main.go"},
			},
			expected: "src/main.go",
		},
		{
			name: "new file uses new name",
			diff: Diff{
				Name: struct {
					Old string `json:"old"`
					New string `json:"new"`
				}{Old: "", New: "src/new.go"},
				IsNew: true,
			},
			expected: "src/new.go",
		},
		{
			name: "deleted file uses old name",
			diff: Diff{
				Name: struct {
					Old string `json:"old"`
					New string `json:"new"`
				}{Old: "src/deleted.go", New: ""},
				IsDelete: true,
			},
			expected: "src/deleted.go",
		},
		{
			name: "renamed file uses new name",
			diff: Diff{
				Name: struct {
					Old string `json:"old"`
					New string `json:"new"`
				}{Old: "src/old.go", New: "src/renamed.go"},
				IsRename: true,
			},
			expected: "src/renamed.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.diff.Id(); got != tt.expected {
				t.Errorf("Diff.Id() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestChangedFilesMatchesDiffId(t *testing.T) {
	// ChangedFiles() must return values matching each Diff's Id()
	// so that sidebar links point to the correct anchors.
	// Tests existing, deleted, new, and renamed files.
	nd := NiceDiff{
		Diff: []Diff{
			{
				Name: struct {
					Old string `json:"old"`
					New string `json:"new"`
				}{Old: "", New: "src/modified.go"},
			},
			{
				Name: struct {
					Old string `json:"old"`
					New string `json:"new"`
				}{Old: "src/deleted.go", New: ""},
				IsDelete: true,
			},
			{
				Name: struct {
					Old string `json:"old"`
					New string `json:"new"`
				}{Old: "", New: "src/new.go"},
				IsNew: true,
			},
			{
				Name: struct {
					Old string `json:"old"`
					New string `json:"new"`
				}{Old: "src/old.go", New: "src/renamed.go"},
				IsRename: true,
			},
		},
	}

	changedFiles := nd.ChangedFiles()

	if len(changedFiles) != len(nd.Diff) {
		t.Fatalf("ChangedFiles() returned %d items, want %d", len(changedFiles), len(nd.Diff))
	}

	for i, diff := range nd.Diff {
		if changedFiles[i] != diff.Id() {
			t.Errorf("ChangedFiles()[%d] = %q, but Diff.Id() = %q", i, changedFiles[i], diff.Id())
		}
	}
}
