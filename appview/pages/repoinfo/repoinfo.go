package repoinfo

import (
	"encoding/json"
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
		{"issues", "/issues", "circle-dot"},
		{"pulls", "/pulls", "git-pull-request"},
		{"pipelines", "/pipelines", "layers-2"},
	}

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
	IsStarred   bool
	Stats       models.RepoStats
	Roles       RolesInRepo
	Source      *models.Repo
	Ref         string
	CurrentDir  string
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

	meta["pulls"] = r.Stats.PullCount.Open
	meta["issues"] = r.Stats.IssueCount.Open

	// more stuff?

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

// PrimaryLanguage returns the first (most used) language from a list, or empty string if none
func PrimaryLanguage(languages []interface{}) string {
	if len(languages) == 0 {
		return ""
	}

	// Languages are already sorted by percentage in descending order
	// Just get the first one
	if firstLang, ok := languages[0].(map[string]interface{}); ok {
		if name, ok := firstLang["Name"].(string); ok {
			return name
		}
	}

	return ""
}

// StructuredData generates Schema.org JSON-LD structured data for the repository
func (r RepoInfo) StructuredData(primaryLanguage string) string {
	data := map[string]interface{}{
		"@context":       "https://schema.org",
		"@type":          "SoftwareSourceCode",
		"name":           r.Name,
		"description":    r.Description,
		"codeRepository": "https://tangled.org/" + r.FullName(),
		"url":            "https://tangled.org/" + r.FullName(),
		"author": map[string]interface{}{
			"@type": "Person",
			"name":  r.owner(),
			"url":   "https://tangled.org/" + r.owner(),
		},
	}

	// Add programming language if available
	if primaryLanguage != "" {
		data["programmingLanguage"] = primaryLanguage
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}
