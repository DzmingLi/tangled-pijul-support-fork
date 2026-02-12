package repoinfo

import (
	"fmt"
	"path"
	"slices"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"tangled.org/core/api/tangled"
	"tangled.org/core/appview/models"
	"tangled.org/core/appview/state/userutil"
)

func (r RepoInfo) owner() string {
	if r.OwnerHandle != "" {
		return r.OwnerHandle
	} else {
		return r.OwnerDid
	}
}

func (r RepoInfo) FullName() string {
	return path.Join(r.owner(), r.Name)
}

func (r RepoInfo) ownerWithoutAt() string {
	if r.OwnerHandle != "" {
		return r.OwnerHandle
	} else {
		return userutil.FlattenDid(r.OwnerDid)
	}
}

func (r RepoInfo) FullNameWithoutAt() string {
	return path.Join(r.ownerWithoutAt(), r.Name)
}

func (r RepoInfo) GetTabs() [][]string {
	tabs := [][]string{
		{"overview", "/", "square-chart-gantt"},
	}

	if r.IsPijul() {
		// Pijul repos use changes and discussions
		tabs = append(tabs, []string{"changes", "/changes", "logs"})
		tabs = append(tabs, []string{"discussions", "/discussions", "message-square"})
	} else {
		// Git repos use separate issues and pulls
		tabs = append(tabs, []string{"issues", "/issues", "circle-dot"})
		tabs = append(tabs, []string{"pulls", "/pulls", "git-pull-request"})
	}

	tabs = append(tabs, []string{"pipelines", "/pipelines", "layers-2"})

	if r.Roles.SettingsAllowed() {
		tabs = append(tabs, []string{"settings", "/settings", "cog"})
	}

	return tabs
}

func (r RepoInfo) RepoAt() syntax.ATURI {
	return syntax.ATURI(fmt.Sprintf("at://%s/%s/%s", r.OwnerDid, tangled.RepoNSID, r.Rkey))
}

type RepoInfo struct {
	Name        string
	Rkey        string
	OwnerDid    string
	OwnerHandle string
	Description string
	Website     string
	Topics      []string
	Knot        string
	Spindle     string
	Vcs         string // "git" or "pijul"
	IsStarred   bool
	Stats       models.RepoStats
	Roles       RolesInRepo
	Source      *models.Repo
	Ref         string
	CurrentDir  string
}

func (r RepoInfo) IsGit() bool {
	return r.Vcs == "" || r.Vcs == "git"
}

func (r RepoInfo) IsPijul() bool {
	return r.Vcs == "pijul"
}

// each tab on a repo could have some metadata:
//
// issues -> number of open issues etc.
// settings -> a warning icon to setup branch protection? idk
//
// we gather these bits of info here, because go templates
// are difficult to program in
func (r RepoInfo) TabMetadata() map[string]any {
	meta := make(map[string]any)

	if r.IsPijul() {
		// Pijul repos use discussions
		meta["discussions"] = r.Stats.DiscussionCount.Open
	} else {
		// Git repos use separate issues and pulls
		meta["issues"] = r.Stats.IssueCount.Open
		meta["pulls"] = r.Stats.PullCount.Open
	}

	return meta
}

type RolesInRepo struct {
	Roles []string
}

func (r RolesInRepo) SettingsAllowed() bool {
	return slices.Contains(r.Roles, "repo:settings")
}

func (r RolesInRepo) CollaboratorInviteAllowed() bool {
	return slices.Contains(r.Roles, "repo:invite")
}

func (r RolesInRepo) RepoDeleteAllowed() bool {
	return slices.Contains(r.Roles, "repo:delete")
}

func (r RolesInRepo) IsOwner() bool {
	return slices.Contains(r.Roles, "repo:owner")
}

func (r RolesInRepo) IsCollaborator() bool {
	return slices.Contains(r.Roles, "repo:collaborator")
}

func (r RolesInRepo) IsPushAllowed() bool {
	return slices.Contains(r.Roles, "repo:push")
}
