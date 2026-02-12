package pijul

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrBinaryFile    = errors.New("binary file")
	ErrNotBinaryFile = errors.New("not binary file")
	ErrNoPijulRepo   = errors.New("not a pijul repository")
	ErrChannelNotFound = errors.New("channel not found")
	ErrChangeNotFound  = errors.New("change not found")
	ErrPathNotFound    = errors.New("path not found")
)

// PijulRepo represents a Pijul repository
type PijulRepo struct {
	path        string
	channelName string // current channel (empty means default)
}

// Open opens a Pijul repository at the given path with optional channel
func Open(path string, channel string) (*PijulRepo, error) {
	// Verify it's a pijul repository
	pijulDir := filepath.Join(path, ".pijul")
	if _, err := os.Stat(pijulDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrNoPijulRepo, path)
	}

	p := &PijulRepo{
		path:        path,
		channelName: channel,
	}

	// Verify channel exists if specified
	if channel != "" {
		channels, err := p.Channels()
		if err != nil {
			return nil, fmt.Errorf("listing channels: %w", err)
		}
		found := false
		for _, ch := range channels {
			if ch.Name == channel {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: %s", ErrChannelNotFound, channel)
		}
	}

	return p, nil
}

// PlainOpen opens a Pijul repository without setting a specific channel
func PlainOpen(path string) (*PijulRepo, error) {
	// Verify it's a pijul repository
	pijulDir := filepath.Join(path, ".pijul")
	if _, err := os.Stat(pijulDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrNoPijulRepo, path)
	}

	return &PijulRepo{path: path}, nil
}

// Path returns the repository path
func (p *PijulRepo) Path() string {
	return p.path
}

// CurrentChannel returns the current channel (or empty for default)
func (p *PijulRepo) CurrentChannel() string {
	return p.channelName
}

// FindDefaultChannel returns the default channel name
func (p *PijulRepo) FindDefaultChannel() (string, error) {
	channels, err := p.Channels()
	if err != nil {
		return "", err
	}

	// Look for 'main' first, then fall back to first channel
	for _, ch := range channels {
		if ch.Name == "main" {
			return "main", nil
		}
	}

	if len(channels) > 0 {
		return channels[0].Name, nil
	}

	return "main", nil // default
}

// SetDefaultChannel changes which channel is considered default
// In Pijul, this would typically be done by renaming channels
func (p *PijulRepo) SetDefaultChannel(channel string) error {
	// Pijul doesn't have a built-in default branch concept like git HEAD
	// This is typically managed at application level
	// For now, just verify the channel exists
	channels, err := p.Channels()
	if err != nil {
		return err
	}

	for _, ch := range channels {
		if ch.Name == channel {
			return nil
		}
	}

	return fmt.Errorf("%w: %s", ErrChannelNotFound, channel)
}

// FileContent reads a file from the working copy at a specific path
// Note: Pijul doesn't have the concept of reading files at a specific revision
// like git. We read from the working directory or need to use pijul credit.
func (p *PijulRepo) FileContent(filePath string) ([]byte, error) {
	fullPath := filepath.Join(p.path, filePath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrPathNotFound, filePath)
		}
		return nil, err
	}

	return content, nil
}

// FileContentN reads up to cap bytes of a file
func (p *PijulRepo) FileContentN(filePath string, cap int64) ([]byte, error) {
	fullPath := filepath.Join(p.path, filePath)

	f, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrPathNotFound, filePath)
		}
		return nil, err
	}
	defer f.Close()

	// Check if binary
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if isBinary(buf[:n]) {
		return nil, ErrBinaryFile
	}

	// Reset and read up to cap
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	content := make([]byte, cap)
	n, err = f.Read(content)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return content[:n], nil
}

// RawContent reads raw file content without binary check
func (p *PijulRepo) RawContent(filePath string) ([]byte, error) {
	fullPath := filepath.Join(p.path, filePath)
	return os.ReadFile(fullPath)
}

// isBinary checks if data appears to be binary
func isBinary(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

// WriteTar writes the repository contents to a tar archive
func (p *PijulRepo) WriteTar(w io.Writer, prefix string) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	return filepath.Walk(p.path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .pijul directory
		if strings.Contains(path, ".pijul") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(p.path, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		header.Name = filepath.Join(prefix, relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}

		return nil
	})
}

// infoWrapper wraps fs.FileInfo for tar operations
type infoWrapper struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (i *infoWrapper) Name() string       { return i.name }
func (i *infoWrapper) Size() int64        { return i.size }
func (i *infoWrapper) Mode() fs.FileMode  { return i.mode }
func (i *infoWrapper) ModTime() time.Time { return i.modTime }
func (i *infoWrapper) IsDir() bool        { return i.isDir }
func (i *infoWrapper) Sys() any           { return nil }

// InitRepo initializes a new Pijul repository
func InitRepo(path string, bare bool) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	args := []string{"init"}
	if bare {
		// Pijul doesn't have explicit bare repos like git
		// A "bare" repo is typically just a repo without a working directory
		args = append(args, "--kind=bare")
	}

	cmd := exec.Command("pijul", args...)
	cmd.Dir = path

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pijul init: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// Clone clones a Pijul repository
func Clone(url, destPath string, channel string) error {
	args := []string{"clone", url, destPath}
	if channel != "" {
		args = append(args, "--channel", channel)
	}

	cmd := exec.Command("pijul", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pijul clone: %w, stderr: %s", err, stderr.String())
	}

	return nil
}
