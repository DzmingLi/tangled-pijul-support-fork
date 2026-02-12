package xrpc

import (
	"net/http"
	"strconv"

	"tangled.org/core/knotserver/git"
	"tangled.org/core/knotserver/pijul"
	"tangled.org/core/types"
	xrpcerr "tangled.org/core/xrpc/errors"
)

func (x *Xrpc) RepoBranches(w http.ResponseWriter, r *http.Request) {
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

	if vcs, _ := pijul.DetectVCS(repoPath); vcs == "pijul" {
		pr, err := pijul.PlainOpen(repoPath)
		if err != nil {
			writeError(w, xrpcerr.RepoNotFoundError, http.StatusNoContent)
			return
		}

		channels, err := pr.ChannelsWithOptions(&pijul.ChannelOptions{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			writeError(w, xrpcerr.GenericError(err), http.StatusInternalServerError)
			return
		}

		branches := make([]types.Branch, 0, len(channels))
		for _, ch := range channels {
			branches = append(branches, types.Branch{
				Reference: types.Reference{Name: ch.Name},
				IsDefault: ch.IsCurrent,
			})
		}

		writeJson(w, types.RepoBranchesResponse{Branches: branches})
		return
	}

	gr, err := git.PlainOpen(repoPath)
	if err != nil {
		writeError(w, xrpcerr.RepoNotFoundError, http.StatusNoContent)
		return
	}

	branches, _ := gr.Branches(&git.BranchesOptions{
		Limit:  limit,
		Offset: offset,
	})

	writeJson(w, types.RepoBranchesResponse{Branches: branches})
}
