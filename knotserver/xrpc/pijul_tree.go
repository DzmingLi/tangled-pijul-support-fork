package xrpc

import (
	"net/http"
	"path/filepath"
	"unicode/utf8"

	"tangled.org/core/api/tangled"
	"tangled.org/core/appview/pages/markup"
	"tangled.org/core/knotserver/pijul"
	xrpcerr "tangled.org/core/xrpc/errors"
)

// RepoPijulTree handles the sh.tangled.repo.pijulTree endpoint
// Returns the file tree for a Pijul repository
func (x *Xrpc) RepoPijulTree(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	repo := r.URL.Query().Get("repo")
	repoPath, err := x.parseRepoParam(repo)
	if err != nil {
		writeError(w, err.(xrpcerr.XrpcError), http.StatusBadRequest)
		return
	}

	channel := r.URL.Query().Get("channel")
	path := r.URL.Query().Get("path")

	pr, err := pijul.Open(repoPath, channel)
	if err != nil {
		x.Logger.Error("failed to open pijul repository", "error", err, "path", repoPath, "channel", channel)
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("RepoNotFound"),
			xrpcerr.WithMessage("failed to open pijul repository"),
		), http.StatusNotFound)
		return
	}

	files, err := pr.FileTree(ctx, path)
	if err != nil {
		x.Logger.Error("failed to get file tree", "error", err, "path", path)
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("PathNotFound"),
			xrpcerr.WithMessage("failed to read repository tree"),
		), http.StatusNotFound)
		return
	}

	// Check for readme file
	var readmeFileName string
	var readmeContents string
	for _, file := range files {
		if markup.IsReadmeFile(file.Name) {
			contents, err := pr.RawContent(filepath.Join(path, file.Name))
			if err != nil {
				x.Logger.Error("failed to read contents of file", "path", path, "file", file.Name)
				continue
			}

			if utf8.Valid(contents) {
				readmeFileName = file.Name
				readmeContents = string(contents)
				break
			}
		}
	}

	// Convert to tangled API format
	treeEntries := make([]*tangled.RepoTree_TreeEntry, len(files))
	for i, file := range files {
		entry := &tangled.RepoTree_TreeEntry{
			Name: file.Name,
			Mode: file.Mode,
			Size: file.Size,
		}
		// Note: LastCommit is not populated for Pijul (would require significant work)
		treeEntries[i] = entry
	}

	var parentPtr *string
	if path != "" {
		parentPtr = &path
	}

	var dotdotPtr *string
	if path != "" {
		dotdot := filepath.Dir(path)
		if dotdot != "." {
			dotdotPtr = &dotdot
		}
	}

	response := tangled.RepoTree_Output{
		Ref:    channel, // Use channel as ref for Pijul
		Parent: parentPtr,
		Dotdot: dotdotPtr,
		Files:  treeEntries,
		Readme: &tangled.RepoTree_Readme{
			Filename: readmeFileName,
			Contents: readmeContents,
		},
	}

	writeJson(w, response)
}

// RepoPijulBlob handles the sh.tangled.repo.pijulBlob endpoint
// Returns file content from a Pijul repository
func (x *Xrpc) RepoPijulBlob(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	repoPath, err := x.parseRepoParam(repo)
	if err != nil {
		writeError(w, err.(xrpcerr.XrpcError), http.StatusBadRequest)
		return
	}

	channel := r.URL.Query().Get("channel")
	path := r.URL.Query().Get("path")

	if path == "" {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("InvalidRequest"),
			xrpcerr.WithMessage("missing path parameter"),
		), http.StatusBadRequest)
		return
	}

	pr, err := pijul.Open(repoPath, channel)
	if err != nil {
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("RepoNotFound"),
			xrpcerr.WithMessage("failed to open pijul repository"),
		), http.StatusNotFound)
		return
	}

	// Try to read as text first
	const maxSize = 1024 * 1024 // 1MB
	content, err := pr.FileContentN(path, maxSize)
	if err != nil {
		if err == pijul.ErrBinaryFile {
			// Return binary indicator
			isBinary := true
			response := tangled.RepoBlob_Output{
				IsBinary: &isBinary,
				Path:     path,
				Ref:      channel,
			}
			writeJson(w, response)
			return
		}

		x.Logger.Error("failed to read file", "error", err, "path", path)
		writeError(w, xrpcerr.NewXrpcError(
			xrpcerr.WithTag("PathNotFound"),
			xrpcerr.WithMessage("failed to read file"),
		), http.StatusNotFound)
		return
	}

	isBinary := false
	contentStr := string(content)
	response := tangled.RepoBlob_Output{
		Content:  &contentStr,
		IsBinary: &isBinary,
		Path:     path,
		Ref:      channel,
	}

	writeJson(w, response)
}
