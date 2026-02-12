package pijul

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"tangled.org/core/types"
)

// TreeEntry represents a file or directory in the repository tree
type TreeEntry struct {
	Name  string      `json:"name"`
	Mode  fs.FileMode `json:"mode"`
	Size  int64       `json:"size"`
	IsDir bool        `json:"is_dir"`
}

// FileTree returns the file tree at the given path
// For Pijul, we read directly from the working directory
func (p *PijulRepo) FileTree(ctx context.Context, treePath string) ([]types.NiceTree, error) {
	fullPath := filepath.Join(p.path, treePath)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPathNotFound
		}
		return nil, err
	}

	// If it's a file, return empty (no tree for files)
	if !info.IsDir() {
		return []types.NiceTree{}, nil
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	trees := make([]types.NiceTree, 0, len(entries))

	for _, entry := range entries {
		// Skip .pijul directory
		if entry.Name() == ".pijul" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		trees = append(trees, types.NiceTree{
			Name: entry.Name(),
			Mode: fileModeToString(info.Mode()),
			Size: info.Size(),
			// LastCommit would require additional work to implement
			// For now, we leave it nil
		})
	}

	return trees, nil
}

// fileModeToString converts fs.FileMode to octal string representation
func fileModeToString(mode fs.FileMode) string {
	// Convert to git-style mode representation
	if mode.IsDir() {
		return "040000"
	}
	if mode&fs.ModeSymlink != 0 {
		return "120000"
	}
	if mode&0111 != 0 {
		return "100755"
	}
	return "100644"
}

// Walk callback type
type WalkCallback func(path string, info fs.FileInfo, isDir bool) error

// Walk traverses the file tree
func (p *PijulRepo) Walk(ctx context.Context, root string, cb WalkCallback) error {
	startPath := filepath.Join(p.path, root)

	return filepath.WalkDir(startPath, func(walkPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip .pijul directory
		if strings.Contains(walkPath, ".pijul") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(p.path, walkPath)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		return cb(relPath, info, d.IsDir())
	})
}

// ListFiles returns all tracked files in the repository
func (p *PijulRepo) ListFiles() ([]string, error) {
	output, err := p.runPijulCmd("ls")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}, nil
	}

	return lines, nil
}

// IsTracked checks if a file is tracked by Pijul
func (p *PijulRepo) IsTracked(filePath string) (bool, error) {
	files, err := p.ListFiles()
	if err != nil {
		return false, err
	}

	for _, f := range files {
		if f == filePath {
			return true, nil
		}
	}

	return false, nil
}

// FileExists checks if a file exists in the working directory
func (p *PijulRepo) FileExists(filePath string) bool {
	fullPath := filepath.Join(p.path, filePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

// IsDir checks if a path is a directory
func (p *PijulRepo) IsDir(treePath string) (bool, error) {
	fullPath := filepath.Join(p.path, treePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// MakeNiceTree creates a NiceTree from file info
func MakeNiceTree(name string, info fs.FileInfo) types.NiceTree {
	return types.NiceTree{
		Name: path.Base(name),
		Mode: fileModeToString(info.Mode()),
		Size: info.Size(),
	}
}
