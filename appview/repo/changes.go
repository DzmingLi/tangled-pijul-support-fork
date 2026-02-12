package repo

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tangled.org/core/api/tangled"
	"tangled.org/core/appview/pages"
	xrpcclient "tangled.org/core/appview/xrpcclient"

	indigoxrpc "github.com/bluesky-social/indigo/xrpc"
	"github.com/go-chi/chi/v5"
)

func (rp *Repo) Changes(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "RepoChanges")

	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to fully resolve repo", "err", err)
		return
	}
	if !f.IsPijul() {
		rp.pages.Error404(w)
		return
	}

	page := 1
	if r.URL.Query().Get("page") != "" {
		page, err = strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil {
			page = 1
		}
	}

	ref := chi.URLParam(r, "ref")
	ref, _ = url.PathUnescape(ref)

	scheme := "http"
	if !rp.config.Core.Dev {
		scheme = "https"
	}
	host := fmt.Sprintf("%s://%s", scheme, f.Knot)
	xrpcc := &indigoxrpc.Client{
		Host: host,
	}

	repo := fmt.Sprintf("%s/%s", f.Did, f.Name)

	if ref == "" {
		channels, err := tangled.RepoChannelList(r.Context(), xrpcc, "", 0, repo)
		if err != nil {
			if xrpcerr := xrpcclient.HandleXrpcErr(err); xrpcerr != nil {
				l.Error("failed to call XRPC repo.channelList", "err", xrpcerr)
				rp.pages.Error503(w)
				return
			}
			rp.pages.Error503(w)
			return
		}
		for _, ch := range channels.Channels {
			if ch.Is_current != nil && *ch.Is_current {
				ref = ch.Name
				break
			}
		}
		if ref == "" && len(channels.Channels) > 0 {
			ref = channels.Channels[0].Name
		}
	}

	limit := int64(60)
	cursor := ""
	if page > 1 {
		offset := (page - 1) * int(limit)
		cursor = strconv.Itoa(offset)
	}

	resp, err := tangled.RepoChangeList(r.Context(), xrpcc, ref, cursor, limit, repo)
	if xrpcerr := xrpcclient.HandleXrpcErr(err); xrpcerr != nil {
		l.Error("failed to call XRPC repo.changeList", "err", xrpcerr)
		rp.pages.Error503(w)
		return
	}

	changes := make([]pages.PijulChangeView, 0, len(resp.Changes))
	for _, change := range resp.Changes {
		view := pages.PijulChangeView{
			Hash:         change.Hash,
			Authors:      change.Authors,
			Message:      change.Message,
			Dependencies: change.Dependencies,
		}
		if change.Timestamp != nil {
			if parsed, err := time.Parse(time.RFC3339, *change.Timestamp); err == nil {
				view.Timestamp = parsed
				view.HasTimestamp = true
			}
		}
		changes = append(changes, view)
	}

	user := rp.oauth.GetMultiAccountUser(r)
	rp.pages.RepoChanges(w, pages.RepoChangesParams{
		LoggedInUser: user,
		RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
		Page:         page,
		Changes:      changes,
	})
}

func (rp *Repo) Change(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "RepoChange")

	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to fully resolve repo", "err", err)
		return
	}
	if !f.IsPijul() {
		rp.pages.Error404(w)
		return
	}

	hash := chi.URLParam(r, "hash")
	if hash == "" {
		rp.pages.Error404(w)
		return
	}

	scheme := "http"
	if !rp.config.Core.Dev {
		scheme = "https"
	}
	host := fmt.Sprintf("%s://%s", scheme, f.Knot)
	xrpcc := &indigoxrpc.Client{
		Host: host,
	}

	repo := fmt.Sprintf("%s/%s", f.Did, f.Name)
	resp, err := tangled.RepoChangeGet(r.Context(), xrpcc, hash, repo)
	if xrpcerr := xrpcclient.HandleXrpcErr(err); xrpcerr != nil {
		l.Error("failed to call XRPC repo.changeGet", "err", xrpcerr)
		rp.pages.Error503(w)
		return
	}

	change := pages.PijulChangeDetail{
		Hash:         resp.Hash,
		Authors:      resp.Authors,
		Message:      resp.Message,
		Dependencies: resp.Dependencies,
	}
	if resp.Diff != nil {
		change.Diff = *resp.Diff
		change.HasDiff = true
		change.DiffLines = parsePijulDiffLines(change.Diff)
	}
	if resp.Timestamp != nil {
		if parsed, err := time.Parse(time.RFC3339, *resp.Timestamp); err == nil {
			change.Timestamp = parsed
			change.HasTimestamp = true
		}
	}

	user := rp.oauth.GetMultiAccountUser(r)
	rp.pages.RepoChange(w, pages.RepoChangeParams{
		LoggedInUser: user,
		RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
		Change:       change,
	})
}

func parsePijulDiffLines(diff string) []pages.PijulDiffLine {
	if diff == "" {
		return nil
	}
	lines := strings.Split(diff, "\n")
	out := make([]pages.PijulDiffLine, 0, len(lines))
	var oldLine int64
	var newLine int64
	var hasOld bool
	var hasNew bool
	for _, line := range lines {
		kind := "context"
		op := " "
		body := line
		switch {
		case strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
			kind = "meta"
			op = ""
			hasOld = false
			hasNew = false
		case strings.HasPrefix(line, "@@"):
			kind = "meta"
			op = ""
			if o, n, ok := parseUnifiedHunkHeader(line); ok {
				oldLine = o
				newLine = n
				hasOld = true
				hasNew = true
			} else {
				hasOld = false
				hasNew = false
			}
		case strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index "):
			kind = "meta"
			op = ""
			hasOld = false
			hasNew = false
		case strings.HasPrefix(line, "#"):
			kind = "section"
			op = ""
			hasOld = false
			hasNew = false
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			kind = "add"
			op = "+"
			body = line[1:]
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			kind = "del"
			op = "-"
			body = line[1:]
		case strings.HasPrefix(line, " "):
			body = line[1:]
		}
		diffLine := pages.PijulDiffLine{
			Kind: kind,
			Op:   op,
			Body: body,
			Text: line,
		}
		if kind != "meta" {
			if kind == "del" {
				if hasOld {
					diffLine.OldLine = oldLine
					diffLine.HasOld = true
					oldLine++
				}
			} else if kind == "add" {
				if hasNew {
					diffLine.NewLine = newLine
					diffLine.HasNew = true
					newLine++
				}
			} else {
				if hasOld {
					diffLine.OldLine = oldLine
					diffLine.HasOld = true
					oldLine++
				}
				if hasNew {
					diffLine.NewLine = newLine
					diffLine.HasNew = true
					newLine++
				}
			}
		}
		out = append(out, diffLine)
	}
	return out
}

func parseUnifiedHunkHeader(line string) (int64, int64, bool) {
	start := strings.Index(line, "@@")
	if start == -1 {
		return 0, 0, false
	}
	trimmed := strings.TrimSpace(line[start+2:])
	end := strings.Index(trimmed, "@@")
	if end == -1 {
		return 0, 0, false
	}
	fields := strings.Fields(strings.TrimSpace(trimmed[:end]))
	if len(fields) < 2 {
		return 0, 0, false
	}
	oldStart, okOld := parseUnifiedRange(fields[0], "-")
	newStart, okNew := parseUnifiedRange(fields[1], "+")
	if !okOld || !okNew {
		return 0, 0, false
	}
	return oldStart, newStart, true
}

func parseUnifiedRange(value, prefix string) (int64, bool) {
	if !strings.HasPrefix(value, prefix) {
		return 0, false
	}
	value = strings.TrimPrefix(value, prefix)
	if value == "" {
		return 0, false
	}
	if idx := strings.Index(value, ","); idx >= 0 {
		value = value[:idx]
	}
	out, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return out, true
}
