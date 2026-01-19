package xrpc

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"

	"tangled.org/core/api/tangled"
	"tangled.org/core/knotserver/git"
	xrpcerr "tangled.org/core/xrpc/errors"
)

func (x *Xrpc) RepoArchive(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	repoPath, err := x.parseRepoParam(repo)
	if err != nil {
		writeError(w, err.(xrpcerr.XrpcError), http.StatusBadRequest)
		return
	}

	ref := r.URL.Query().Get("ref")
	// ref can be empty (git.Open handles this)

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "tar.gz" // default
	}

	prefix := r.URL.Query().Get("prefix")

	if format != "tar.gz" {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InvalidRequest"),
			xrpcerr.WithMessage("only tar.gz format is supported"),
		), http.StatusBadRequest)
		return
	}

	gr, err := git.Open(repoPath, ref)
	if err != nil {
		writeError(w, xrpcerr.RefNotFoundError, http.StatusNotFound)
		return
	}

	repoParts := strings.Split(repo, "/")
	repoName := repoParts[len(repoParts)-1]

	immutableLink, err := x.buildImmutableLink(repo, format, gr.Hash().String(), prefix)
	if err != nil {
		x.Logger.Error(
			"failed to build immutable link",
			"err", err.Error(),
			"repo", repo,
			"format", format,
			"ref", gr.Hash().String(),
			"prefix", prefix,
		)
	}

	safeRefFilename := strings.ReplaceAll(plumbing.ReferenceName(ref).Short(), "/", "-")

	var archivePrefix string
	if prefix != "" {
		archivePrefix = prefix
	} else {
		archivePrefix = fmt.Sprintf("%s-%s", repoName, safeRefFilename)
	}

	filename := fmt.Sprintf("%s-%s.tar.gz", repoName, safeRefFilename)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"immutable\"", immutableLink))

	gw := gzip.NewWriter(w)
	defer gw.Close()

	err = gr.WriteTar(gw, archivePrefix)
	if err != nil {
		// once we start writing to the body we can't report error anymore
		// so we are only left with logging the error
		x.Logger.Error("writing tar file", "error", err.Error())
		return
	}

	err = gw.Flush()
	if err != nil {
		// once we start writing to the body we can't report error anymore
		// so we are only left with logging the error
		x.Logger.Error("flushing", "error", err.Error())
		return
	}
}

func (x *Xrpc) buildImmutableLink(repo string, format string, ref string, prefix string) (string, error) {
	scheme := "https"
	if x.Config.Server.Dev {
		scheme = "http"
	}

	u, err := url.Parse(scheme + "://" + x.Config.Server.Hostname + "/xrpc/" + tangled.RepoArchiveNSID)
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("repo", repo)
	params.Set("format", format)
	params.Set("ref", ref)
	params.Set("prefix", prefix)

	return fmt.Sprintf("%s?%s", u.String(), params.Encode()), nil
}
