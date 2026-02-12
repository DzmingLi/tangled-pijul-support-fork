package xrpc

import (
	"net/http"
	"strconv"
	"time"

	"tangled.org/core/knotserver/pijul"
	xrpcerr "tangled.org/core/xrpc/errors"
)

// PijulChangeListResponse is the response for listing Pijul changes
type PijulChangeListResponse struct {
	Changes []PijulChangeEntry `json:"changes"`
	Channel string             `json:"channel,omitempty"`
	Page    int                `json:"page"`
	PerPage int                `json:"per_page"`
	Total   int                `json:"total"`
}

// PijulChangeEntry represents a single change in the list
type PijulChangeEntry struct {
	Hash         string              `json:"hash"`
	Authors      []PijulAuthor       `json:"authors"`
	Message      string              `json:"message"`
	Timestamp    string              `json:"timestamp,omitempty"`
	Dependencies []string            `json:"dependencies,omitempty"`
}

// PijulAuthor represents a change author
type PijulAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// RepoChangeList handles the sh.tangled.repo.changeList endpoint
// Lists changes (Pijul equivalent of commits) in a repository
func (x *Xrpc) RepoChangeList(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	repoPath, err := x.parseRepoParam(repo)
	if err != nil {
		writeError(w, err.(xrpcerr.XrpcError), http.StatusBadRequest)
		return
	}

	channel := r.URL.Query().Get("channel")
	cursor := r.URL.Query().Get("cursor")

	limit := 50 // default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	pr, err := pijul.Open(repoPath, channel)
	if err != nil {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("RepoNotFound"),
			xrpcerr.WithMessage("failed to open pijul repository"),
		), http.StatusNotFound)
		return
	}

	offset := 0
	if cursor != "" {
		if o, err := strconv.Atoi(cursor); err == nil && o >= 0 {
			offset = o
		}
	}

	changes, err := pr.Changes(offset, limit)
	if err != nil {
		x.Logger.Error("fetching changes", "error", err.Error())
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InternalServerError"),
			xrpcerr.WithMessage("failed to read change log"),
		), http.StatusInternalServerError)
		return
	}

	total, err := pr.TotalChanges()
	if err != nil {
		x.Logger.Error("fetching total changes", "error", err.Error())
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InternalServerError"),
			xrpcerr.WithMessage("failed to fetch total changes"),
		), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	changeEntries := make([]PijulChangeEntry, len(changes))
	for i, c := range changes {
		authors := make([]PijulAuthor, len(c.Authors))
		for j, a := range c.Authors {
			authors[j] = PijulAuthor{
				Name:  a.Name,
				Email: a.Email,
			}
		}

		changeEntries[i] = PijulChangeEntry{
			Hash:         c.Hash,
			Authors:      authors,
			Message:      c.Message,
			Dependencies: c.Dependencies,
		}

		if !c.Timestamp.IsZero() {
			changeEntries[i].Timestamp = c.Timestamp.Format(time.RFC3339)
		}
	}

	response := PijulChangeListResponse{
		Changes: changeEntries,
		Channel: channel,
		Page:    (offset / limit) + 1,
		PerPage: limit,
		Total:   total,
	}

	writeJson(w, response)
}

// PijulChangeGetResponse is the response for getting a single change
type PijulChangeGetResponse struct {
	Hash         string        `json:"hash"`
	Authors      []PijulAuthor `json:"authors"`
	Message      string        `json:"message"`
	Timestamp    string        `json:"timestamp,omitempty"`
	Dependencies []string      `json:"dependencies,omitempty"`
	Diff         string        `json:"diff,omitempty"`
}

// RepoChangeGet handles the sh.tangled.repo.changeGet endpoint
// Gets details for a specific change
func (x *Xrpc) RepoChangeGet(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	repoPath, err := x.parseRepoParam(repo)
	if err != nil {
		writeError(w, err.(xrpcerr.XrpcError), http.StatusBadRequest)
		return
	}

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InvalidRequest"),
			xrpcerr.WithMessage("missing hash parameter"),
		), http.StatusBadRequest)
		return
	}

	pr, err := pijul.PlainOpen(repoPath)
	if err != nil {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("RepoNotFound"),
			xrpcerr.WithMessage("failed to open pijul repository"),
		), http.StatusNotFound)
		return
	}

	change, err := pr.GetChange(hash)
	if err != nil {
		x.Logger.Error("fetching change", "error", err.Error(), "hash", hash)
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("ChangeNotFound"),
			xrpcerr.WithMessage("change not found"),
		), http.StatusNotFound)
		return
	}

	// Get diff for this change
	diff, _ := pr.DiffChange(hash)

	authors := make([]PijulAuthor, len(change.Authors))
	for i, a := range change.Authors {
		authors[i] = PijulAuthor{
			Name:  a.Name,
			Email: a.Email,
		}
	}

	response := PijulChangeGetResponse{
		Hash:         change.Hash,
		Authors:      authors,
		Message:      change.Message,
		Dependencies: change.Dependencies,
	}

	if !change.Timestamp.IsZero() {
		response.Timestamp = change.Timestamp.Format(time.RFC3339)
	}

	if diff != nil {
		response.Diff = diff.Raw
	}

	writeJson(w, response)
}
