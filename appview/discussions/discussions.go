package discussions

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/go-chi/chi/v5"

	tangled "tangled.org/core/api/tangled"
	"tangled.org/core/appview/config"
	"tangled.org/core/appview/db"
	"tangled.org/core/appview/mentions"
	"tangled.org/core/appview/models"
	"tangled.org/core/appview/notify"
	"tangled.org/core/appview/oauth"
	"tangled.org/core/appview/pages"
	"tangled.org/core/appview/pagination"
	"tangled.org/core/appview/reporesolver"
	"tangled.org/core/appview/validator"
	"tangled.org/core/idresolver"
	"tangled.org/core/orm"
	"tangled.org/core/rbac"
	"tangled.org/core/tid"
)

// Discussions handles the discussions feature for Pijul repositories
type Discussions struct {
	oauth            *oauth.OAuth
	repoResolver     *reporesolver.RepoResolver
	enforcer         *rbac.Enforcer
	pages            *pages.Pages
	idResolver       *idresolver.Resolver
	mentionsResolver *mentions.Resolver
	db               *db.DB
	config           *config.Config
	notifier         notify.Notifier
	logger           *slog.Logger
	validator        *validator.Validator
}

func New(
	oauth *oauth.OAuth,
	repoResolver *reporesolver.RepoResolver,
	enforcer *rbac.Enforcer,
	pages *pages.Pages,
	idResolver *idresolver.Resolver,
	mentionsResolver *mentions.Resolver,
	db *db.DB,
	config *config.Config,
	notifier notify.Notifier,
	validator *validator.Validator,
	logger *slog.Logger,
) *Discussions {
	return &Discussions{
		oauth:            oauth,
		repoResolver:     repoResolver,
		enforcer:         enforcer,
		pages:            pages,
		idResolver:       idResolver,
		mentionsResolver: mentionsResolver,
		db:               db,
		config:           config,
		notifier:         notifier,
		logger:           logger,
		validator:        validator,
	}
}

// RepoDiscussionsList shows all discussions for a Pijul repository
func (d *Discussions) RepoDiscussionsList(w http.ResponseWriter, r *http.Request) {
	l := d.logger.With("handler", "RepoDiscussionsList")
	user := d.oauth.GetMultiAccountUser(r)

	repo, err := d.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo", "err", err)
		d.pages.Error404(w)
		return
	}

	// Only allow discussions for Pijul repos
	if !repo.IsPijul() {
		l.Info("discussions only available for pijul repos")
		d.pages.Error404(w)
		return
	}

	repoAt := repo.RepoAt()
	page := pagination.Page{Limit: 50}

	// Filter by state
	filter := r.URL.Query().Get("filter")
	filters := []orm.Filter{orm.FilterEq("repo_at", repoAt)}
	switch filter {
	case "closed":
		filters = append(filters, orm.FilterEq("state", models.DiscussionClosed))
	case "merged":
		filters = append(filters, orm.FilterEq("state", models.DiscussionMerged))
	default:
		// Default to open
		filters = append(filters, orm.FilterEq("state", models.DiscussionOpen))
		filter = "open"
	}

	discussions, err := db.GetDiscussionsPaginated(d.db, page, filters...)
	if err != nil {
		l.Error("failed to fetch discussions", "err", err)
		d.pages.Error503(w)
		return
	}

	count, err := db.GetDiscussionCount(d.db, repoAt)
	if err != nil {
		l.Error("failed to get discussion count", "err", err)
	}

	d.pages.RepoDiscussionsList(w, pages.RepoDiscussionsListParams{
		LoggedInUser:    user,
		RepoInfo:        d.repoResolver.GetRepoInfo(r, user),
		Discussions:     discussions,
		Filter:          filter,
		DiscussionCount: count,
	})
}

// NewDiscussion creates a new discussion
func (d *Discussions) NewDiscussion(w http.ResponseWriter, r *http.Request) {
	l := d.logger.With("handler", "NewDiscussion")
	user := d.oauth.GetMultiAccountUser(r)

	repo, err := d.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo", "err", err)
		d.pages.Error404(w)
		return
	}

	if !repo.IsPijul() {
		l.Info("discussions only available for pijul repos")
		d.pages.Error404(w)
		return
	}

	repoInfo := d.repoResolver.GetRepoInfo(r, user)

	switch r.Method {
	case http.MethodGet:
		d.pages.NewDiscussion(w, pages.NewDiscussionParams{
			LoggedInUser: user,
			RepoInfo:     repoInfo,
		})

	case http.MethodPost:
		noticeId := "discussion"

		title := r.FormValue("title")
		body := r.FormValue("body")
		targetChannel := r.FormValue("target_channel")
		if targetChannel == "" {
			targetChannel = "main"
		}

		if title == "" {
			d.pages.Notice(w, noticeId, "Title is required")
			return
		}

		discussion := &models.Discussion{
			Did:           user.Active.Did,
			Rkey:          tid.TID(),
			RepoAt:        repo.RepoAt(),
			Title:         title,
			Body:          body,
			TargetChannel: targetChannel,
			State:         models.DiscussionOpen,
			Created:       time.Now(),
		}

		tx, err := d.db.BeginTx(r.Context(), nil)
		if err != nil {
			l.Error("failed to begin transaction", "err", err)
			d.pages.Notice(w, noticeId, "Failed to create discussion")
			return
		}
		defer tx.Rollback()

		if err := db.NewDiscussion(tx, discussion); err != nil {
			l.Error("failed to create discussion", "err", err)
			d.pages.Notice(w, noticeId, "Failed to create discussion")
			return
		}

		if err := tx.Commit(); err != nil {
			l.Error("failed to commit transaction", "err", err)
			d.pages.Notice(w, noticeId, "Failed to create discussion")
			return
		}

		// Subscribe the creator to the discussion
		db.SubscribeToDiscussion(d.db, discussion.AtUri(), user.Active.Did)

		l.Info("discussion created", "discussion_id", discussion.DiscussionId)

		d.pages.HxLocation(w, fmt.Sprintf("/%s/%s/discussions/%d",
			user.Active.Did, repo.Name, discussion.DiscussionId))
	}
}

// RepoSingleDiscussion shows a single discussion
func (d *Discussions) RepoSingleDiscussion(w http.ResponseWriter, r *http.Request) {
	l := d.logger.With("handler", "RepoSingleDiscussion")
	user := d.oauth.GetMultiAccountUser(r)

	discussion, ok := r.Context().Value("discussion").(*models.Discussion)
	if !ok {
		l.Error("failed to get discussion from context")
		d.pages.Error404(w)
		return
	}

	repoInfo := d.repoResolver.GetRepoInfo(r, user)

	// Check if user can manage patches
	canManage := false
	if user != nil {
		roles := d.enforcer.GetPermissionsInRepo(user.Active.Did, repoInfo.Knot, repoInfo.OwnerDid+"/"+repoInfo.Name)
		for _, role := range roles {
			if role == "repo:push" || role == "repo:owner" || role == "repo:collaborator" {
				canManage = true
				break
			}
		}
	}

	d.pages.RepoSingleDiscussion(w, pages.RepoSingleDiscussionParams{
		LoggedInUser:  user,
		RepoInfo:      repoInfo,
		Discussion:    discussion,
		CommentList:   discussion.CommentList(),
		CanManage:     canManage,
		ActivePatches: discussion.ActivePatches(),
	})
}

// AddPatch allows anyone to add a patch to a discussion
func (d *Discussions) AddPatch(w http.ResponseWriter, r *http.Request) {
	l := d.logger.With("handler", "AddPatch")
	user := d.oauth.GetMultiAccountUser(r)
	noticeId := "patch"

	discussion, ok := r.Context().Value("discussion").(*models.Discussion)
	if !ok {
		l.Error("failed to get discussion from context")
		d.pages.Notice(w, noticeId, "Discussion not found")
		return
	}

	if discussion.State != models.DiscussionOpen {
		d.pages.Notice(w, noticeId, "Cannot add patches to a closed or merged discussion")
		return
	}

	patchHash := r.FormValue("patch_hash")
	patch := r.FormValue("patch")

	if patchHash == "" || patch == "" {
		d.pages.Notice(w, noticeId, "Patch hash and content are required")
		return
	}

	// Check if patch already exists
	exists, err := db.PatchExists(d.db, discussion.AtUri(), patchHash)
	if err != nil {
		l.Error("failed to check patch existence", "err", err)
		d.pages.Notice(w, noticeId, "Failed to add patch")
		return
	}
	if exists {
		d.pages.Notice(w, noticeId, "This patch has already been added to the discussion")
		return
	}

	// Get repo info for verification and dependency checking
	repo, err := d.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to resolve repo", "err", err)
		d.pages.Notice(w, noticeId, "Failed to add patch")
		return
	}

	repoIdentifier := fmt.Sprintf("%s/%s", repo.Did, repo.Name)

	// Verify the change exists in the Pijul repository
	change, err := d.getChangeFromKnot(r.Context(), repo.Knot, repoIdentifier, patchHash)
	if err != nil {
		l.Info("change verification failed", "hash", patchHash, "err", err)
		d.pages.Notice(w, noticeId, "Change not found in repository. Please ensure the change hash is correct and exists in the repo.")
		return
	}

	l.Debug("change verified", "hash", patchHash, "message", change.Message)

	// Check dependencies - ensure the patch doesn't depend on removed patches
	if err := d.canAddPatchWithChange(discussion, change); err != nil {
		l.Info("dependency check failed", "err", err)
		d.pages.Notice(w, noticeId, err.Error())
		return
	}

	discussionPatch := &models.DiscussionPatch{
		DiscussionAt: discussion.AtUri(),
		PushedByDid:  user.Active.Did,
		PatchHash:    patchHash,
		Patch:        patch,
		Added:        time.Now(),
	}

	tx, err := d.db.BeginTx(r.Context(), nil)
	if err != nil {
		l.Error("failed to begin transaction", "err", err)
		d.pages.Notice(w, noticeId, "Failed to add patch")
		return
	}
	defer tx.Rollback()

	if err := db.AddDiscussionPatch(tx, discussionPatch); err != nil {
		l.Error("failed to add patch", "err", err)
		d.pages.Notice(w, noticeId, "Failed to add patch")
		return
	}

	if err := tx.Commit(); err != nil {
		l.Error("failed to commit transaction", "err", err)
		d.pages.Notice(w, noticeId, "Failed to add patch")
		return
	}

	// Subscribe the patch contributor to the discussion
	db.SubscribeToDiscussion(d.db, discussion.AtUri(), user.Active.Did)

	l.Info("patch added", "patch_hash", patchHash, "pushed_by", user.Active.Did)

	// Reload the page to show the new patch
	d.pages.HxLocation(w, fmt.Sprintf("/%s/%s/discussions/%d",
		repo.Did, repo.Name, discussion.DiscussionId))
}

// RemovePatch removes a patch from a discussion (soft delete)
func (d *Discussions) RemovePatch(w http.ResponseWriter, r *http.Request) {
	l := d.logger.With("handler", "RemovePatch")
	user := d.oauth.GetMultiAccountUser(r)
	noticeId := "patch"

	discussion, ok := r.Context().Value("discussion").(*models.Discussion)
	if !ok {
		l.Error("failed to get discussion from context")
		d.pages.Notice(w, noticeId, "Discussion not found")
		return
	}

	patchIdStr := chi.URLParam(r, "patchId")
	patchId, err := strconv.ParseInt(patchIdStr, 10, 64)
	if err != nil {
		d.pages.Notice(w, noticeId, "Invalid patch ID")
		return
	}

	patch, err := db.GetDiscussionPatch(d.db, patchId)
	if err != nil {
		l.Error("failed to get patch", "err", err)
		d.pages.Notice(w, noticeId, "Patch not found")
		return
	}

	// Check permission: patch pusher or repo collaborator
	repoInfo := d.repoResolver.GetRepoInfo(r, user)
	canRemove := patch.PushedByDid == user.Active.Did
	if !canRemove {
		roles := d.enforcer.GetPermissionsInRepo(user.Active.Did, repoInfo.Knot, repoInfo.OwnerDid+"/"+repoInfo.Name)
		for _, role := range roles {
			if role == "repo:push" || role == "repo:owner" || role == "repo:collaborator" {
				canRemove = true
				break
			}
		}
	}

	if !canRemove {
		d.pages.Notice(w, noticeId, "You don't have permission to remove this patch")
		return
	}

	// Get repo for dependency checking
	repo, err := d.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to resolve repo", "err", err)
		d.pages.Notice(w, noticeId, "Failed to remove patch")
		return
	}

	// Check if other active patches depend on this one
	repoIdentifier := fmt.Sprintf("%s/%s", repo.Did, repo.Name)
	if err := d.canRemovePatch(r.Context(), discussion, repo.Knot, repoIdentifier, patch.PatchHash); err != nil {
		l.Info("dependency check failed", "err", err)
		d.pages.Notice(w, noticeId, err.Error())
		return
	}

	if err := db.RemovePatch(d.db, patchId); err != nil {
		l.Error("failed to remove patch", "err", err)
		d.pages.Notice(w, noticeId, "Failed to remove patch")
		return
	}

	l.Info("patch removed", "patch_id", patchId)

	d.pages.HxLocation(w, fmt.Sprintf("/%s/%s/discussions/%d",
		repo.Did, repo.Name, discussion.DiscussionId))
}

// ReaddPatch re-adds a previously removed patch
func (d *Discussions) ReaddPatch(w http.ResponseWriter, r *http.Request) {
	l := d.logger.With("handler", "ReaddPatch")
	user := d.oauth.GetMultiAccountUser(r)
	noticeId := "patch"

	discussion, ok := r.Context().Value("discussion").(*models.Discussion)
	if !ok {
		l.Error("failed to get discussion from context")
		d.pages.Notice(w, noticeId, "Discussion not found")
		return
	}

	patchIdStr := chi.URLParam(r, "patchId")
	patchId, err := strconv.ParseInt(patchIdStr, 10, 64)
	if err != nil {
		d.pages.Notice(w, noticeId, "Invalid patch ID")
		return
	}

	patch, err := db.GetDiscussionPatch(d.db, patchId)
	if err != nil {
		l.Error("failed to get patch", "err", err)
		d.pages.Notice(w, noticeId, "Patch not found")
		return
	}

	// Check permission
	repoInfo := d.repoResolver.GetRepoInfo(r, user)
	canReadd := patch.PushedByDid == user.Active.Did
	if !canReadd {
		roles := d.enforcer.GetPermissionsInRepo(user.Active.Did, repoInfo.Knot, repoInfo.OwnerDid+"/"+repoInfo.Name)
		for _, role := range roles {
			if role == "repo:push" || role == "repo:owner" || role == "repo:collaborator" {
				canReadd = true
				break
			}
		}
	}

	if !canReadd {
		d.pages.Notice(w, noticeId, "You don't have permission to re-add this patch")
		return
	}

	if err := db.ReaddPatch(d.db, patchId); err != nil {
		l.Error("failed to re-add patch", "err", err)
		d.pages.Notice(w, noticeId, "Failed to re-add patch")
		return
	}

	l.Info("patch re-added", "patch_id", patchId)

	repo, _ := d.repoResolver.Resolve(r)
	d.pages.HxLocation(w, fmt.Sprintf("/%s/%s/discussions/%d",
		repo.Did, repo.Name, discussion.DiscussionId))
}

// NewComment adds a comment to a discussion
func (d *Discussions) NewComment(w http.ResponseWriter, r *http.Request) {
	l := d.logger.With("handler", "NewComment")
	user := d.oauth.GetMultiAccountUser(r)
	noticeId := "comment"

	discussion, ok := r.Context().Value("discussion").(*models.Discussion)
	if !ok {
		l.Error("failed to get discussion from context")
		d.pages.Notice(w, noticeId, "Discussion not found")
		return
	}

	body := r.FormValue("body")
	replyTo := r.FormValue("reply_to")

	if body == "" {
		d.pages.Notice(w, noticeId, "Comment body is required")
		return
	}

	comment := models.DiscussionComment{
		Did:          user.Active.Did,
		Rkey:         tid.TID(),
		DiscussionAt: discussion.AtUri().String(),
		Body:         body,
		Created:      time.Now(),
	}

	if replyTo != "" {
		comment.ReplyTo = &replyTo
	}

	tx, err := d.db.BeginTx(r.Context(), nil)
	if err != nil {
		l.Error("failed to begin transaction", "err", err)
		d.pages.Notice(w, noticeId, "Failed to add comment")
		return
	}
	defer tx.Rollback()

	if _, err := db.AddDiscussionComment(tx, comment); err != nil {
		l.Error("failed to add comment", "err", err)
		d.pages.Notice(w, noticeId, "Failed to add comment")
		return
	}

	if err := tx.Commit(); err != nil {
		l.Error("failed to commit transaction", "err", err)
		d.pages.Notice(w, noticeId, "Failed to add comment")
		return
	}

	// Subscribe the commenter to the discussion
	db.SubscribeToDiscussion(d.db, discussion.AtUri(), user.Active.Did)

	l.Info("comment added", "discussion_id", discussion.DiscussionId)

	repo, _ := d.repoResolver.Resolve(r)
	d.pages.HxLocation(w, fmt.Sprintf("/%s/%s/discussions/%d",
		repo.Did, repo.Name, discussion.DiscussionId))
}

// CloseDiscussion closes a discussion
func (d *Discussions) CloseDiscussion(w http.ResponseWriter, r *http.Request) {
	l := d.logger.With("handler", "CloseDiscussion")
	user := d.oauth.GetMultiAccountUser(r)
	noticeId := "discussion"

	discussion, ok := r.Context().Value("discussion").(*models.Discussion)
	if !ok {
		l.Error("failed to get discussion from context")
		d.pages.Notice(w, noticeId, "Discussion not found")
		return
	}

	// Check permission
	repoInfo := d.repoResolver.GetRepoInfo(r, user)
	canClose := discussion.Did == user.Active.Did
	if !canClose {
		roles := d.enforcer.GetPermissionsInRepo(user.Active.Did, repoInfo.Knot, repoInfo.OwnerDid+"/"+repoInfo.Name)
		for _, role := range roles {
			if role == "repo:push" || role == "repo:owner" || role == "repo:collaborator" {
				canClose = true
				break
			}
		}
	}

	if !canClose {
		d.pages.Notice(w, noticeId, "You don't have permission to close this discussion")
		return
	}

	if err := db.CloseDiscussion(d.db, discussion.RepoAt, discussion.DiscussionId); err != nil {
		l.Error("failed to close discussion", "err", err)
		d.pages.Notice(w, noticeId, "Failed to close discussion")
		return
	}

	l.Info("discussion closed", "discussion_id", discussion.DiscussionId)

	repo, _ := d.repoResolver.Resolve(r)
	d.pages.HxLocation(w, fmt.Sprintf("/%s/%s/discussions/%d",
		repo.Did, repo.Name, discussion.DiscussionId))
}

// ReopenDiscussion reopens a discussion
func (d *Discussions) ReopenDiscussion(w http.ResponseWriter, r *http.Request) {
	l := d.logger.With("handler", "ReopenDiscussion")
	user := d.oauth.GetMultiAccountUser(r)
	noticeId := "discussion"

	discussion, ok := r.Context().Value("discussion").(*models.Discussion)
	if !ok {
		l.Error("failed to get discussion from context")
		d.pages.Notice(w, noticeId, "Discussion not found")
		return
	}

	// Check permission
	repoInfo := d.repoResolver.GetRepoInfo(r, user)
	canReopen := discussion.Did == user.Active.Did
	if !canReopen {
		roles := d.enforcer.GetPermissionsInRepo(user.Active.Did, repoInfo.Knot, repoInfo.OwnerDid+"/"+repoInfo.Name)
		for _, role := range roles {
			if role == "repo:push" || role == "repo:owner" || role == "repo:collaborator" {
				canReopen = true
				break
			}
		}
	}

	if !canReopen {
		d.pages.Notice(w, noticeId, "You don't have permission to reopen this discussion")
		return
	}

	if err := db.ReopenDiscussion(d.db, discussion.RepoAt, discussion.DiscussionId); err != nil {
		l.Error("failed to reopen discussion", "err", err)
		d.pages.Notice(w, noticeId, "Failed to reopen discussion")
		return
	}

	l.Info("discussion reopened", "discussion_id", discussion.DiscussionId)

	repo, _ := d.repoResolver.Resolve(r)
	d.pages.HxLocation(w, fmt.Sprintf("/%s/%s/discussions/%d",
		repo.Did, repo.Name, discussion.DiscussionId))
}

// MergeDiscussion applies patches and marks a discussion as merged
func (d *Discussions) MergeDiscussion(w http.ResponseWriter, r *http.Request) {
	l := d.logger.With("handler", "MergeDiscussion")
	user := d.oauth.GetMultiAccountUser(r)
	noticeId := "discussion"

	discussion, ok := r.Context().Value("discussion").(*models.Discussion)
	if !ok {
		l.Error("failed to get discussion from context")
		d.pages.Notice(w, noticeId, "Discussion not found")
		return
	}

	// Only collaborators can merge
	repoInfo := d.repoResolver.GetRepoInfo(r, user)
	canMerge := false
	roles := d.enforcer.GetPermissionsInRepo(user.Active.Did, repoInfo.Knot, repoInfo.OwnerDid+"/"+repoInfo.Name)
	for _, role := range roles {
		if role == "repo:push" || role == "repo:owner" || role == "repo:collaborator" {
			canMerge = true
			break
		}
	}

	if !canMerge {
		d.pages.Notice(w, noticeId, "You don't have permission to merge this discussion")
		return
	}

	// Get all active patches to apply
	activePatches := discussion.ActivePatches()
	if len(activePatches) == 0 {
		d.pages.Notice(w, noticeId, "No patches to merge")
		return
	}

	// Get repo for API call
	repo, err := d.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to resolve repo", "err", err)
		d.pages.Notice(w, noticeId, "Failed to merge discussion")
		return
	}

	// Apply patches via knotserver
	scheme := "http"
	if d.config.Core.UseTLS() {
		scheme = "https"
	}
	host := fmt.Sprintf("%s://%s", scheme, repo.Knot)

	xrpcc := &xrpc.Client{
		Host: host,
	}

	// Collect patch hashes in order
	changeHashes := make([]string, len(activePatches))
	for i, patch := range activePatches {
		changeHashes[i] = patch.PatchHash
	}

	repoIdentifier := fmt.Sprintf("%s/%s", repo.Did, repo.Name)
	applyInput := &tangled.RepoApplyChanges_Input{
		Repo:    repoIdentifier,
		Channel: discussion.TargetChannel,
		Changes: changeHashes,
	}

	applyResult, err := tangled.RepoApplyChanges(r.Context(), xrpcc, applyInput)
	if err != nil {
		l.Error("failed to apply changes", "err", err)
		d.pages.Notice(w, noticeId, "Failed to apply patches: "+err.Error())
		return
	}

	// Check if all patches were applied
	if len(applyResult.Failed) > 0 {
		failedHashes := make([]string, len(applyResult.Failed))
		for i, f := range applyResult.Failed {
			failedHashes[i] = f.Hash[:12]
		}
		l.Warn("some patches failed to apply", "failed", failedHashes)
		d.pages.Notice(w, noticeId, fmt.Sprintf("Some patches failed to apply: %v", failedHashes))
		return
	}

	l.Info("patches applied successfully", "count", len(applyResult.Applied))

	// Mark discussion as merged
	if err := db.MergeDiscussion(d.db, discussion.RepoAt, discussion.DiscussionId); err != nil {
		l.Error("failed to merge discussion", "err", err)
		d.pages.Notice(w, noticeId, "Failed to merge discussion")
		return
	}

	l.Info("discussion merged", "discussion_id", discussion.DiscussionId)

	repo, _ = d.repoResolver.Resolve(r)
	d.pages.HxLocation(w, fmt.Sprintf("/%s/%s/discussions/%d",
		repo.Did, repo.Name, discussion.DiscussionId))
}

// getChangeFromKnot fetches change details (including dependencies) from knotserver
func (d *Discussions) getChangeFromKnot(ctx context.Context, knot, repo, hash string) (*tangled.RepoChangeGet_Output, error) {
	scheme := "http"
	if d.config.Core.UseTLS() {
		scheme = "https"
	}
	host := fmt.Sprintf("%s://%s", scheme, knot)

	xrpcc := &xrpc.Client{
		Host: host,
	}

	return tangled.RepoChangeGet(ctx, xrpcc, hash, repo)
}

// canAddPatchWithChange checks if a patch can be added to the discussion
// Uses the already-fetched change object to avoid duplicate API calls
// Returns error if the patch depends on a removed patch
func (d *Discussions) canAddPatchWithChange(discussion *models.Discussion, change *tangled.RepoChangeGet_Output) error {

	if len(change.Dependencies) == 0 {
		return nil // No dependencies, can always add
	}

	// Get all patches in this discussion
	patches, err := db.GetDiscussionPatches(d.db, orm.FilterEq("discussion_at", discussion.AtUri()))
	if err != nil {
		return fmt.Errorf("failed to get discussion patches: %w", err)
	}

	// Check if any dependency is a removed patch in this discussion
	for _, dep := range change.Dependencies {
		for _, patch := range patches {
			if patch.PatchHash == dep && !patch.IsActive() {
				return fmt.Errorf("cannot add patch: it depends on removed patch %s", dep[:12])
			}
		}
	}

	return nil
}

// canRemovePatch checks if a patch can be removed from the discussion
// Returns error if other active patches depend on this patch
func (d *Discussions) canRemovePatch(ctx context.Context, discussion *models.Discussion, knot, repo, patchHashToRemove string) error {
	// Get all active patches in this discussion
	patches, err := db.GetDiscussionPatches(d.db, orm.FilterEq("discussion_at", discussion.AtUri()))
	if err != nil {
		return fmt.Errorf("failed to get discussion patches: %w", err)
	}

	// For each active patch, check if it depends on the patch we want to remove
	for _, patch := range patches {
		if !patch.IsActive() || patch.PatchHash == patchHashToRemove {
			continue
		}

		// Get the change details to check its dependencies
		change, err := d.getChangeFromKnot(ctx, knot, repo, patch.PatchHash)
		if err != nil {
			d.logger.Warn("failed to get change dependencies", "hash", patch.PatchHash, "err", err)
			continue // Skip if we can't get the change, but don't block removal
		}

		for _, dep := range change.Dependencies {
			if dep == patchHashToRemove {
				return fmt.Errorf("cannot remove patch: patch %s depends on it", patch.PatchHash[:12])
			}
		}
	}

	return nil
}
