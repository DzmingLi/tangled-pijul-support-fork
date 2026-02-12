package pijul

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// InitBare initializes a bare Pijul repository
func InitBare(repoPath string) error {
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	cmd := exec.Command("pijul", "init")
	cmd.Dir = repoPath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pijul init: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// IsPijulRepo checks if the given path is a Pijul repository
func IsPijulRepo(path string) bool {
	pijulDir := filepath.Join(path, ".pijul")
	info, err := os.Stat(pijulDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsGitRepo checks if the given path is a Git repository
func IsGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		// Also check for bare git repos (where .git is the repo itself)
		headFile := filepath.Join(path, "HEAD")
		if _, err := os.Stat(headFile); err == nil {
			return true
		}
		return false
	}
	return info.IsDir()
}

// DetectVCS detects whether a path contains a Git or Pijul repository
func DetectVCS(path string) (string, error) {
	if IsPijulRepo(path) {
		return "pijul", nil
	}
	if IsGitRepo(path) {
		return "git", nil
	}
	return "", fmt.Errorf("no VCS repository found at %s", path)
}

// Fork clones a repository to a new location
func Fork(srcPath, destPath string) error {
	// For local fork, we can use pijul clone
	cmd := exec.Command("pijul", "clone", srcPath, destPath)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pijul clone: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// Push pushes changes to a remote
func (p *PijulRepo) Push(remote string, channel string) error {
	args := []string{"push", remote}
	if channel != "" {
		args = append(args, "--channel", channel)
	}

	_, err := p.runPijulCmd("push", args...)
	return err
}

// Pull pulls changes from a remote
func (p *PijulRepo) Pull(remote string, channel string) error {
	args := []string{"pull", remote}
	if channel != "" {
		args = append(args, "--channel", channel)
	}

	_, err := p.runPijulCmd("pull", args...)
	return err
}

// Apply applies a change to the repository
func (p *PijulRepo) Apply(changeHash string) error {
	_, err := p.runPijulCmd("apply", changeHash)
	return err
}

// Unrecord removes a change from the channel (like git reset)
func (p *PijulRepo) Unrecord(changeHash string) error {
	args := []string{changeHash}
	if p.channelName != "" {
		args = append(args, "--channel", p.channelName)
	}
	_, err := p.runPijulCmd("unrecord", args...)
	return err
}

// Record creates a new change (like git commit)
func (p *PijulRepo) Record(message string, authors []Author) error {
	args := []string{"-m", message}

	for _, author := range authors {
		authorStr := author.Name
		if author.Email != "" {
			authorStr = fmt.Sprintf("%s <%s>", author.Name, author.Email)
		}
		args = append(args, "--author", authorStr)
	}

	if p.channelName != "" {
		args = append(args, "--channel", p.channelName)
	}

	_, err := p.runPijulCmd("record", args...)
	return err
}

// Add adds files to be tracked
func (p *PijulRepo) Add(paths ...string) error {
	args := append([]string{}, paths...)
	_, err := p.runPijulCmd("add", args...)
	return err
}

// Remove removes files from tracking
func (p *PijulRepo) Remove(paths ...string) error {
	args := append([]string{}, paths...)
	_, err := p.runPijulCmd("remove", args...)
	return err
}
