package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

type RepoSuite struct {
	t        *testing.T
	tempDir  string
	repo     *GitRepo
	baseTime time.Time
}

func NewRepoSuite(t *testing.T) *RepoSuite {
	tempDir, err := os.MkdirTemp("", "git-test-*")
	require.NoError(t, err)

	return &RepoSuite{
		t:        t,
		tempDir:  tempDir,
		baseTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (h *RepoSuite) cleanup() {
	if h.tempDir != "" {
		os.RemoveAll(h.tempDir)
	}
}

func (h *RepoSuite) init() *GitRepo {
	repoPath := filepath.Join(h.tempDir, "test-repo")

	// initialize repository
	r, err := gogit.PlainInit(repoPath, false)
	require.NoError(h.t, err)

	// configure git user
	cfg, err := r.Config()
	require.NoError(h.t, err)
	cfg.User.Name = "Test User"
	cfg.User.Email = "test@example.com"
	err = r.SetConfig(cfg)
	require.NoError(h.t, err)

	// create initial commit with a file
	w, err := r.Worktree()
	require.NoError(h.t, err)

	// create initial file
	initialFile := filepath.Join(repoPath, "README.md")
	err = os.WriteFile(initialFile, []byte("# Test Repository\n\nInitial content.\n"), 0644)
	require.NoError(h.t, err)

	_, err = w.Add("README.md")
	require.NoError(h.t, err)

	_, err = w.Commit("Initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  h.baseTime,
		},
	})
	require.NoError(h.t, err)

	gitRepo, err := PlainOpen(repoPath)
	require.NoError(h.t, err)

	h.repo = gitRepo
	return gitRepo
}

func (h *RepoSuite) commitFile(filename, content, message string) plumbing.Hash {
	filePath := filepath.Join(h.repo.path, filename)
	dir := filepath.Dir(filePath)

	err := os.MkdirAll(dir, 0755)
	require.NoError(h.t, err)

	err = os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(h.t, err)

	w, err := h.repo.r.Worktree()
	require.NoError(h.t, err)

	_, err = w.Add(filename)
	require.NoError(h.t, err)

	hash, err := w.Commit(message, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(h.t, err)

	return hash
}

func (h *RepoSuite) createAnnotatedTag(name string, commit plumbing.Hash, taggerName, taggerEmail, message string, when time.Time) {
	_, err := h.repo.r.CreateTag(name, commit, &gogit.CreateTagOptions{
		Tagger: &object.Signature{
			Name:  taggerName,
			Email: taggerEmail,
			When:  when,
		},
		Message: message,
	})
	require.NoError(h.t, err)
}

func (h *RepoSuite) createLightweightTag(name string, commit plumbing.Hash) {
	ref := plumbing.NewReferenceFromStrings("refs/tags/"+name, commit.String())
	err := h.repo.r.Storer.SetReference(ref)
	require.NoError(h.t, err)
}

func (h *RepoSuite) createBranch(name string, commit plumbing.Hash) {
	ref := plumbing.NewReferenceFromStrings("refs/heads/"+name, commit.String())
	err := h.repo.r.Storer.SetReference(ref)
	require.NoError(h.t, err)
}

func (h *RepoSuite) checkoutBranch(name string) {
	w, err := h.repo.r.Worktree()
	require.NoError(h.t, err)

	err = w.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(name),
	})
	require.NoError(h.t, err)
}
