package git

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	knotconfig "tangled.org/core/knotserver/config"
)

func Fork(repoPath, source string, cfg *knotconfig.Config) error {
	u, err := url.Parse(source)
	if err != nil {
		return fmt.Errorf("failed to parse source URL: %w", err)
	}

	if o := optimizeClone(u, cfg); o != nil {
		u = o
	}

	cloneCmd := exec.Command("git", "clone", "--bare", u.String(), repoPath)
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("failed to bare clone repository: %w", err)
	}

	configureCmd := exec.Command("git", "-C", repoPath, "config", "receive.hideRefs", "refs/hidden")
	if err := configureCmd.Run(); err != nil {
		return fmt.Errorf("failed to configure hidden refs: %w", err)
	}

	return nil
}

func optimizeClone(u *url.URL, cfg *knotconfig.Config) *url.URL {
	// only optimize if it's the same host
	if u.Host != cfg.Server.Hostname {
		return nil
	}

	local := filepath.Join(cfg.Repo.ScanPath, u.Path)

	// sanity check: is there a git repo there?
	if _, err := PlainOpen(local); err != nil {
		return nil
	}

	// create optimized file:// URL
	optimized := &url.URL{
		Scheme: "file",
		Path:   local,
	}

	slog.Debug("performing local clone", "url", optimized.String())
	return optimized
}

func (g *GitRepo) Sync() error {
	branch := g.h.String()

	fetchOpts := &git.FetchOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec("+" + branch + ":" + branch), // +refs/heads/master:refs/heads/master
		},
	}

	err := g.r.Fetch(fetchOpts)
	if errors.Is(git.NoErrAlreadyUpToDate, err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to fetch origin branch: %s: %w", branch, err)
	}
	return nil
}

// TrackHiddenRemoteRef tracks a hidden remote in the repository. For example,
// if the feature branch on the fork (forkRef) is feature-1, and the remoteRef,
// i.e. the branch we want to merge into, is main, this will result in a refspec:
//
//	+refs/heads/main:refs/hidden/feature-1/main
func (g *GitRepo) TrackHiddenRemoteRef(forkRef, remoteRef string) error {
	fetchOpts := &git.FetchOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/hidden/%s/%s", remoteRef, forkRef, remoteRef)),
		},
		RemoteName: "origin",
	}

	err := g.r.Fetch(fetchOpts)
	if errors.Is(git.NoErrAlreadyUpToDate, err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to fetch hidden remote: %s: %w", forkRef, err)
	}
	return nil
}
