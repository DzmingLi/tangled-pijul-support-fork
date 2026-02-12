package pijul

import (
	"fmt"
)

// Diff represents the difference between two states
type Diff struct {
	Raw     string      `json:"raw"`
	Files   []FileDiff  `json:"files,omitempty"`
	Stats   *DiffStats  `json:"stats,omitempty"`
}

// FileDiff represents changes to a single file
type FileDiff struct {
	Path      string `json:"path"`
	OldPath   string `json:"old_path,omitempty"` // for renames
	Status    string `json:"status"`             // added, modified, deleted, renamed
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch,omitempty"`
}

// DiffStats contains summary statistics for a diff
type DiffStats struct {
	FilesChanged int `json:"files_changed"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
}

// Diff returns the diff of uncommitted changes
func (p *PijulRepo) Diff() (*Diff, error) {
	output, err := p.diff()
	if err != nil {
		return nil, fmt.Errorf("pijul diff: %w", err)
	}

	return &Diff{
		Raw: string(output),
	}, nil
}

// DiffChange returns the diff for a specific change
func (p *PijulRepo) DiffChange(hash string) (*Diff, error) {
	output, err := p.change(hash)
	if err != nil {
		return nil, fmt.Errorf("pijul change %s: %w", hash, err)
	}

	return &Diff{
		Raw: string(output),
	}, nil
}

// DiffBetween returns the diff between two channels or states
func (p *PijulRepo) DiffBetween(from, to string) (*Diff, error) {
	args := []string{}
	if from != "" {
		args = append(args, "--channel", from)
	}
	if to != "" {
		args = append(args, "--channel", to)
	}

	output, err := p.diff(args...)
	if err != nil {
		return nil, fmt.Errorf("pijul diff: %w", err)
	}

	return &Diff{
		Raw: string(output),
	}, nil
}
