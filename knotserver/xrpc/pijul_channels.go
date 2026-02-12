package xrpc

import (
	"net/http"
	"strconv"

	"tangled.org/core/knotserver/pijul"
	xrpcerr "tangled.org/core/xrpc/errors"
)

// PijulChannelListResponse is the response for listing Pijul channels
type PijulChannelListResponse struct {
	Channels []PijulChannel `json:"channels"`
}

// PijulChannel represents a Pijul channel
type PijulChannel struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"is_current,omitempty"`
}

// RepoChannelList handles the sh.tangled.repo.channelList endpoint
// Lists channels (Pijul equivalent of branches) in a repository
func (x *Xrpc) RepoChannelList(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	repoPath, err := x.parseRepoParam(repo)
	if err != nil {
		writeError(w, err.(xrpcerr.XrpcError), http.StatusBadRequest)
		return
	}

	// default
	limit := 50
	offset := 0

	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	if o, err := strconv.Atoi(r.URL.Query().Get("cursor")); err == nil && o > 0 {
		offset = o
	}

	pr, err := pijul.PlainOpen(repoPath)
	if err != nil {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("RepoNotFound"),
			xrpcerr.WithMessage("failed to open pijul repository"),
		), http.StatusNotFound)
		return
	}

	channels, err := pr.ChannelsWithOptions(&pijul.ChannelOptions{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		x.Logger.Error("fetching channels", "error", err.Error())
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InternalServerError"),
			xrpcerr.WithMessage("failed to list channels"),
		), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	channelList := make([]PijulChannel, len(channels))
	for i, ch := range channels {
		channelList[i] = PijulChannel{
			Name:      ch.Name,
			IsCurrent: ch.IsCurrent,
		}
	}

	response := PijulChannelListResponse{
		Channels: channelList,
	}

	writeJson(w, response)
}

// PijulGetDefaultChannelResponse is the response for getting the default channel
type PijulGetDefaultChannelResponse struct {
	Channel string `json:"channel"`
}

// RepoGetDefaultChannel handles the sh.tangled.repo.getDefaultChannel endpoint
func (x *Xrpc) RepoGetDefaultChannel(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	repoPath, err := x.parseRepoParam(repo)
	if err != nil {
		writeError(w, err.(xrpcerr.XrpcError), http.StatusBadRequest)
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

	channel, err := pr.FindDefaultChannel()
	if err != nil {
		x.Logger.Error("finding default channel", "error", err.Error())
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InternalServerError"),
			xrpcerr.WithMessage("failed to find default channel"),
		), http.StatusInternalServerError)
		return
	}

	response := PijulGetDefaultChannelResponse{
		Channel: channel,
	}

	writeJson(w, response)
}
