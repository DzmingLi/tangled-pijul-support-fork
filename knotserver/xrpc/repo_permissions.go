package xrpc

import (
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	securejoin "github.com/cyphar/filepath-securejoin"
	"tangled.org/core/api/tangled"
	"tangled.org/core/knotserver/pijul"
	"tangled.org/core/rbac"
	xrpcerr "tangled.org/core/xrpc/errors"
)

const (
	permRead int64 = 1 << iota
	permCreateDiscussion
	permEditDiscussion
	permTagDiscussion
	permApply
	permEditChannels
	permEditTags
	permEditPermissions
)

func (x *Xrpc) RepoPermissions(w http.ResponseWriter, r *http.Request) {
	l := x.Logger.With("handler", "RepoPermissions")
	fail := func(e xrpcerr.XrpcError, status int) {
		l.Error("failed", "kind", e.Tag, "error", e.Message)
		writeError(w, e, status)
	}

	actorDid, ok := r.Context().Value(ActorDid).(syntax.DID)
	if !ok {
		fail(xrpcerr.MissingActorDidError, http.StatusBadRequest)
		return
	}

	repo := r.URL.Query().Get("repo")
	if repo == "" {
		fail(xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InvalidRequest"),
			xrpcerr.WithMessage("missing repo parameter"),
		), http.StatusBadRequest)
		return
	}

	repoPath, err := x.parseRepoParam(repo)
	if err != nil {
		fail(err.(xrpcerr.XrpcError), http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(repoPath); err != nil {
		if os.IsNotExist(err) {
			writeError(w, xrpcerr.RepoNotFoundError, http.StatusNoContent)
			return
		}
		fail(xrpcerr.GenericError(err), http.StatusInternalServerError)
		return
	}

	if vcs, _ := pijul.DetectVCS(repoPath); vcs != "pijul" {
		fail(xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InvalidRequest"),
			xrpcerr.WithMessage("permissions are only available for pijul repositories"),
		), http.StatusBadRequest)
		return
	}

	repoParts := strings.SplitN(repo, "/", 2)
	if len(repoParts) != 2 {
		fail(xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InvalidRequest"),
			xrpcerr.WithMessage("invalid repo format, expected 'did/repoName'"),
		), http.StatusBadRequest)
		return
	}

	didSlashRepo, err := securejoin.SecureJoin(repoParts[0], repoParts[1])
	if err != nil {
		fail(xrpcerr.InvalidRepoError(repo), http.StatusBadRequest)
		return
	}

	roles := x.Enforcer.GetPermissionsInRepo(actorDid.String(), rbac.ThisServer, didSlashRepo)

	var (
		canRead             bool
		canCreateDiscussion bool
		canEditDiscussion   bool
		canTagDiscussion    bool
		canApply            bool
		canEditChannels     bool
		canEditTags         bool
		canEditPermissions  bool
	)

	canRead = slices.Contains(roles, rbac.PijulRead)
	canCreateDiscussion = slices.Contains(roles, rbac.PijulCreateDiscussion)
	canEditDiscussion = slices.Contains(roles, rbac.PijulEditDiscussion)
	canTagDiscussion = slices.Contains(roles, rbac.PijulTagDiscussion)
	canApply = slices.Contains(roles, rbac.PijulApply)
	canEditChannels = slices.Contains(roles, rbac.PijulEditChannels)
	canEditTags = slices.Contains(roles, rbac.PijulEditTags)
	canEditPermissions = slices.Contains(roles, rbac.PijulEditPermissions)

	var mask int64
	var permissions []string
	add := func(name string, bit int64, ok bool) {
		if !ok {
			return
		}
		mask |= bit
		permissions = append(permissions, name)
	}

	add("read", permRead, canRead)
	add("create_discussion", permCreateDiscussion, canCreateDiscussion)
	add("edit_discussion", permEditDiscussion, canEditDiscussion)
	add("tag_discussion", permTagDiscussion, canTagDiscussion)
	add("apply", permApply, canApply)
	add("edit_channels", permEditChannels, canEditChannels)
	add("edit_tags", permEditTags, canEditTags)
	add("edit_permissions", permEditPermissions, canEditPermissions)

	writeJson(w, tangled.RepoPermissions_Output{
		Mask:        mask,
		Permissions: permissions,
	})
}
