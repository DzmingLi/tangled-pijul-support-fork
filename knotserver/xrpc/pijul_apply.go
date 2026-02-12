package xrpc

import (
	"encoding/json"
	"net/http"

	"tangled.org/core/knotserver/pijul"
	xrpcerr "tangled.org/core/xrpc/errors"
)

// ApplyChangesRequest is the request body for applying changes
type ApplyChangesRequest struct {
	Repo    string   `json:"repo"`
	Channel string   `json:"channel"`
	Changes []string `json:"changes"`
}

// ApplyChangesResponse is the response for applying changes
type ApplyChangesResponse struct {
	Applied []string              `json:"applied"`
	Failed  []ApplyChangeFailure `json:"failed,omitempty"`
}

// ApplyChangeFailure represents a failed change application
type ApplyChangeFailure struct {
	Hash  string `json:"hash"`
	Error string `json:"error"`
}

// RepoApplyChanges handles the sh.tangled.repo.applyChanges endpoint
// Applies Pijul changes to a repository channel (used for merging discussions)
func (x *Xrpc) RepoApplyChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InvalidRequest"),
			xrpcerr.WithMessage("method not allowed"),
		), http.StatusMethodNotAllowed)
		return
	}

	var req ApplyChangesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InvalidRequest"),
			xrpcerr.WithMessage("invalid request body"),
		), http.StatusBadRequest)
		return
	}

	if req.Repo == "" || req.Channel == "" || len(req.Changes) == 0 {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InvalidRequest"),
			xrpcerr.WithMessage("repo, channel, and changes are required"),
		), http.StatusBadRequest)
		return
	}

	repoPath, err := x.parseRepoParam(req.Repo)
	if err != nil {
		writeError(w, err.(xrpcerr.XrpcError), http.StatusBadRequest)
		return
	}

	// Open the repository with the target channel
	pr, err := pijul.Open(repoPath, req.Channel)
	if err != nil {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("RepoNotFound"),
			xrpcerr.WithMessage("failed to open pijul repository"),
		), http.StatusNotFound)
		return
	}

	// Verify the channel exists
	channels, err := pr.Channels()
	if err != nil {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InternalServerError"),
			xrpcerr.WithMessage("failed to list channels"),
		), http.StatusInternalServerError)
		return
	}

	channelExists := false
	for _, ch := range channels {
		if ch.Name == req.Channel {
			channelExists = true
			break
		}
	}

	if !channelExists {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("ChannelNotFound"),
			xrpcerr.WithMessage("target channel not found"),
		), http.StatusNotFound)
		return
	}

	// Apply each change in order
	response := ApplyChangesResponse{
		Applied: make([]string, 0),
		Failed:  make([]ApplyChangeFailure, 0),
	}

	for _, changeHash := range req.Changes {
		if err := pr.Apply(changeHash); err != nil {
			x.Logger.Error("failed to apply change", "hash", changeHash, "error", err.Error())
			response.Failed = append(response.Failed, ApplyChangeFailure{
				Hash:  changeHash,
				Error: err.Error(),
			})
		} else {
			response.Applied = append(response.Applied, changeHash)
			x.Logger.Info("applied change", "hash", changeHash, "channel", req.Channel)
		}
	}

	// If any changes failed, return partial success
	if len(response.Failed) > 0 && len(response.Applied) == 0 {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("ApplyFailed"),
			xrpcerr.WithMessage("all changes failed to apply"),
		), http.StatusInternalServerError)
		return
	}

	writeJson(w, response)
}
