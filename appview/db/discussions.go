package db

import (
	"database/sql"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"tangled.org/core/appview/models"
	"tangled.org/core/appview/pagination"
	"tangled.org/core/orm"
)

// NewDiscussion creates a new discussion in a Pijul repository
func NewDiscussion(tx *sql.Tx, discussion *models.Discussion) error {
	// ensure sequence exists
	_, err := tx.Exec(`
		insert or ignore into repo_discussion_seqs (repo_at, next_discussion_id)
		values (?, 1)
	`, discussion.RepoAt)
	if err != nil {
		return err
	}

	// get next discussion_id
	var newDiscussionId int
	err = tx.QueryRow(`
		update repo_discussion_seqs
		set next_discussion_id = next_discussion_id + 1
		where repo_at = ?
		returning next_discussion_id - 1
	`, discussion.RepoAt).Scan(&newDiscussionId)
	if err != nil {
		return err
	}

	// insert new discussion
	row := tx.QueryRow(`
		insert into discussions (repo_at, did, rkey, discussion_id, title, body, target_channel, state)
		values (?, ?, ?, ?, ?, ?, ?, ?)
		returning id, discussion_id
	`, discussion.RepoAt, discussion.Did, discussion.Rkey, newDiscussionId, discussion.Title, discussion.Body, discussion.TargetChannel, discussion.State)

	err = row.Scan(&discussion.Id, &discussion.DiscussionId)
	if err != nil {
		return fmt.Errorf("scan row: %w", err)
	}

	return nil
}

// GetDiscussionsPaginated returns discussions with pagination
func GetDiscussionsPaginated(e Execer, page pagination.Page, filters ...orm.Filter) ([]models.Discussion, error) {
	discussionMap := make(map[string]*models.Discussion) // at-uri -> discussion

	var conditions []string
	var args []any

	for _, filter := range filters {
		conditions = append(conditions, filter.Condition())
		args = append(args, filter.Arg()...)
	}

	whereClause := ""
	if conditions != nil {
		whereClause = " where " + strings.Join(conditions, " and ")
	}

	pLower := orm.FilterGte("row_num", page.Offset+1)
	pUpper := orm.FilterLte("row_num", page.Offset+page.Limit)

	pageClause := ""
	if page.Limit > 0 {
		args = append(args, pLower.Arg()...)
		args = append(args, pUpper.Arg()...)
		pageClause = " where " + pLower.Condition() + " and " + pUpper.Condition()
	}

	query := fmt.Sprintf(
		`
		select * from (
			select
				id,
				did,
				rkey,
				repo_at,
				discussion_id,
				title,
				body,
				target_channel,
				state,
				created,
				edited,
				row_number() over (order by created desc) as row_num
			from
				discussions
			%s
		) ranked_discussions
		%s
		`,
		whereClause,
		pageClause,
	)

	rows, err := e.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query discussions table: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var discussion models.Discussion
		var createdAt string
		var editedAt sql.Null[string]
		var rowNum int64
		err := rows.Scan(
			&discussion.Id,
			&discussion.Did,
			&discussion.Rkey,
			&discussion.RepoAt,
			&discussion.DiscussionId,
			&discussion.Title,
			&discussion.Body,
			&discussion.TargetChannel,
			&discussion.State,
			&createdAt,
			&editedAt,
			&rowNum,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan discussion: %w", err)
		}

		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			discussion.Created = t
		}

		if editedAt.Valid {
			if t, err := time.Parse(time.RFC3339, editedAt.V); err == nil {
				discussion.Edited = &t
			}
		}

		atUri := discussion.AtUri().String()
		discussionMap[atUri] = &discussion
	}

	// collect reverse repos
	repoAts := make([]string, 0, len(discussionMap))
	for _, discussion := range discussionMap {
		repoAts = append(repoAts, string(discussion.RepoAt))
	}

	repos, err := GetRepos(e, 0, orm.FilterIn("at_uri", repoAts))
	if err != nil {
		return nil, fmt.Errorf("failed to build repo mappings: %w", err)
	}

	repoMap := make(map[string]*models.Repo)
	for i := range repos {
		repoMap[string(repos[i].RepoAt())] = &repos[i]
	}

	for discussionAt, d := range discussionMap {
		if r, ok := repoMap[string(d.RepoAt)]; ok {
			d.Repo = r
		} else {
			// do not show up the discussion if the repo is deleted
			delete(discussionMap, discussionAt)
		}
	}

	// collect patches
	discussionAts := slices.Collect(maps.Keys(discussionMap))

	patches, err := GetDiscussionPatches(e, orm.FilterIn("discussion_at", discussionAts))
	if err != nil {
		return nil, fmt.Errorf("failed to query patches: %w", err)
	}
	for _, p := range patches {
		discussionAt := p.DiscussionAt.String()
		if discussion, ok := discussionMap[discussionAt]; ok {
			discussion.Patches = append(discussion.Patches, p)
		}
	}

	// collect comments
	comments, err := GetDiscussionComments(e, orm.FilterIn("discussion_at", discussionAts))
	if err != nil {
		return nil, fmt.Errorf("failed to query comments: %w", err)
	}
	for i := range comments {
		discussionAt := comments[i].DiscussionAt
		if discussion, ok := discussionMap[discussionAt]; ok {
			discussion.Comments = append(discussion.Comments, comments[i])
		}
	}

	// collect labels for each discussion
	allLabels, err := GetLabels(e, orm.FilterIn("subject", discussionAts))
	if err != nil {
		return nil, fmt.Errorf("failed to query labels: %w", err)
	}
	for discussionAt, labels := range allLabels {
		if discussion, ok := discussionMap[discussionAt.String()]; ok {
			discussion.Labels = labels
		}
	}

	var discussions []models.Discussion
	for _, d := range discussionMap {
		discussions = append(discussions, *d)
	}

	sort.Slice(discussions, func(i, j int) bool {
		return discussions[i].Created.After(discussions[j].Created)
	})

	return discussions, nil
}

// GetDiscussion returns a single discussion by repo and ID
func GetDiscussion(e Execer, repoAt syntax.ATURI, discussionId int) (*models.Discussion, error) {
	discussions, err := GetDiscussionsPaginated(
		e,
		pagination.Page{},
		orm.FilterEq("repo_at", repoAt),
		orm.FilterEq("discussion_id", discussionId),
	)
	if err != nil {
		return nil, err
	}
	if len(discussions) != 1 {
		return nil, sql.ErrNoRows
	}

	return &discussions[0], nil
}

// GetDiscussions returns discussions matching filters
func GetDiscussions(e Execer, filters ...orm.Filter) ([]models.Discussion, error) {
	return GetDiscussionsPaginated(e, pagination.Page{}, filters...)
}

// AddDiscussionPatch adds a patch to a discussion
// Anyone can add patches - the key feature of the Nest model
func AddDiscussionPatch(tx *sql.Tx, patch *models.DiscussionPatch) error {
	row := tx.QueryRow(`
		insert into discussion_patches (discussion_at, pushed_by_did, patch_hash, patch)
		values (?, ?, ?, ?)
		returning id
	`, patch.DiscussionAt, patch.PushedByDid, patch.PatchHash, patch.Patch)

	return row.Scan(&patch.Id)
}

// PatchExists checks if a patch with the given hash already exists in the discussion
func PatchExists(e Execer, discussionAt syntax.ATURI, patchHash string) (bool, error) {
	var count int
	err := e.QueryRow(`
		select count(1) from discussion_patches
		where discussion_at = ? and patch_hash = ?
	`, discussionAt, patchHash).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RemovePatch marks a patch as removed (soft delete)
func RemovePatch(e Execer, patchId int64) error {
	_, err := e.Exec(`
		update discussion_patches
		set removed = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		where id = ?
	`, patchId)
	return err
}

// ReaddPatch re-adds a previously removed patch
func ReaddPatch(e Execer, patchId int64) error {
	_, err := e.Exec(`
		update discussion_patches
		set removed = null
		where id = ?
	`, patchId)
	return err
}

// GetDiscussionPatches returns patches for discussions
func GetDiscussionPatches(e Execer, filters ...orm.Filter) ([]*models.DiscussionPatch, error) {
	var conditions []string
	var args []any
	for _, filter := range filters {
		conditions = append(conditions, filter.Condition())
		args = append(args, filter.Arg()...)
	}

	whereClause := ""
	if conditions != nil {
		whereClause = " where " + strings.Join(conditions, " and ")
	}

	query := fmt.Sprintf(`
		select
			id,
			discussion_at,
			pushed_by_did,
			patch_hash,
			patch,
			added,
			removed
		from
			discussion_patches
		%s
		order by added asc
	`, whereClause)

	rows, err := e.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patches []*models.DiscussionPatch
	for rows.Next() {
		var patch models.DiscussionPatch
		var addedAt string
		var removedAt sql.Null[string]
		err := rows.Scan(
			&patch.Id,
			&patch.DiscussionAt,
			&patch.PushedByDid,
			&patch.PatchHash,
			&patch.Patch,
			&addedAt,
			&removedAt,
		)
		if err != nil {
			return nil, err
		}

		if t, err := time.Parse(time.RFC3339, addedAt); err == nil {
			patch.Added = t
		}

		if removedAt.Valid {
			if t, err := time.Parse(time.RFC3339, removedAt.V); err == nil {
				patch.Removed = &t
			}
		}

		patches = append(patches, &patch)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return patches, nil
}

// GetDiscussionPatch returns a single patch by ID
func GetDiscussionPatch(e Execer, patchId int64) (*models.DiscussionPatch, error) {
	patches, err := GetDiscussionPatches(e, orm.FilterEq("id", patchId))
	if err != nil {
		return nil, err
	}
	if len(patches) != 1 {
		return nil, sql.ErrNoRows
	}
	return patches[0], nil
}

// AddDiscussionComment adds a comment to a discussion
func AddDiscussionComment(tx *sql.Tx, c models.DiscussionComment) (int64, error) {
	result, err := tx.Exec(
		`insert into discussion_comments (
			did,
			rkey,
			discussion_at,
			body,
			reply_to,
			created,
			edited
		)
		values (?, ?, ?, ?, ?, ?, null)
		on conflict(did, rkey) do update set
			discussion_at = excluded.discussion_at,
			body = excluded.body,
			edited = case
				when
					discussion_comments.discussion_at != excluded.discussion_at
					or discussion_comments.body != excluded.body
					or discussion_comments.reply_to != excluded.reply_to
				then ?
				else discussion_comments.edited
			end`,
		c.Did,
		c.Rkey,
		c.DiscussionAt,
		c.Body,
		c.ReplyTo,
		c.Created.Format(time.RFC3339),
		time.Now().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// GetDiscussionComments returns comments for discussions
func GetDiscussionComments(e Execer, filters ...orm.Filter) ([]models.DiscussionComment, error) {
	var conditions []string
	var args []any
	for _, filter := range filters {
		conditions = append(conditions, filter.Condition())
		args = append(args, filter.Arg()...)
	}

	whereClause := ""
	if conditions != nil {
		whereClause = " where " + strings.Join(conditions, " and ")
	}

	query := fmt.Sprintf(`
		select
			id,
			did,
			rkey,
			discussion_at,
			reply_to,
			body,
			created,
			edited,
			deleted
		from
			discussion_comments
		%s
		order by created asc
	`, whereClause)

	rows, err := e.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.DiscussionComment
	for rows.Next() {
		var comment models.DiscussionComment
		var created string
		var rkey, edited, deleted, replyTo sql.Null[string]
		err := rows.Scan(
			&comment.Id,
			&comment.Did,
			&rkey,
			&comment.DiscussionAt,
			&replyTo,
			&comment.Body,
			&created,
			&edited,
			&deleted,
		)
		if err != nil {
			return nil, err
		}

		if rkey.Valid {
			comment.Rkey = rkey.V
		}

		if t, err := time.Parse(time.RFC3339, created); err == nil {
			comment.Created = t
		}

		if edited.Valid {
			if t, err := time.Parse(time.RFC3339, edited.V); err == nil {
				comment.Edited = &t
			}
		}

		if deleted.Valid {
			if t, err := time.Parse(time.RFC3339, deleted.V); err == nil {
				comment.Deleted = &t
			}
		}

		if replyTo.Valid {
			comment.ReplyTo = &replyTo.V
		}

		comments = append(comments, comment)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return comments, nil
}

// DeleteDiscussionComment soft-deletes a comment
func DeleteDiscussionComment(e Execer, filters ...orm.Filter) error {
	var conditions []string
	var args []any
	for _, filter := range filters {
		conditions = append(conditions, filter.Condition())
		args = append(args, filter.Arg()...)
	}

	whereClause := ""
	if conditions != nil {
		whereClause = " where " + strings.Join(conditions, " and ")
	}

	query := fmt.Sprintf(`update discussion_comments set body = "", deleted = strftime('%%Y-%%m-%%dT%%H:%%M:%%SZ', 'now') %s`, whereClause)

	_, err := e.Exec(query, args...)
	return err
}

// CloseDiscussion closes a discussion
func CloseDiscussion(e Execer, repoAt syntax.ATURI, discussionId int) error {
	_, err := e.Exec(`
		update discussions set state = ?
		where repo_at = ? and discussion_id = ?
	`, models.DiscussionClosed, repoAt, discussionId)
	return err
}

// ReopenDiscussion reopens a discussion
func ReopenDiscussion(e Execer, repoAt syntax.ATURI, discussionId int) error {
	_, err := e.Exec(`
		update discussions set state = ?
		where repo_at = ? and discussion_id = ?
	`, models.DiscussionOpen, repoAt, discussionId)
	return err
}

// MergeDiscussion marks a discussion as merged
func MergeDiscussion(e Execer, repoAt syntax.ATURI, discussionId int) error {
	_, err := e.Exec(`
		update discussions set state = ?
		where repo_at = ? and discussion_id = ?
	`, models.DiscussionMerged, repoAt, discussionId)
	return err
}

// SetDiscussionState sets the state of a discussion
func SetDiscussionState(e Execer, repoAt syntax.ATURI, discussionId int, state models.DiscussionState) error {
	_, err := e.Exec(`
		update discussions set state = ?
		where repo_at = ? and discussion_id = ?
	`, state, repoAt, discussionId)
	return err
}

// GetDiscussionCount returns counts of discussions by state
func GetDiscussionCount(e Execer, repoAt syntax.ATURI) (models.DiscussionCount, error) {
	row := e.QueryRow(`
		select
			count(case when state = ? then 1 end) as open_count,
			count(case when state = ? then 1 end) as merged_count,
			count(case when state = ? then 1 end) as closed_count
		from discussions
		where repo_at = ?`,
		models.DiscussionOpen,
		models.DiscussionMerged,
		models.DiscussionClosed,
		repoAt,
	)

	var count models.DiscussionCount
	if err := row.Scan(&count.Open, &count.Merged, &count.Closed); err != nil {
		return models.DiscussionCount{}, err
	}

	return count, nil
}

// SubscribeToDiscussion adds a subscription for a user to a discussion
func SubscribeToDiscussion(e Execer, discussionAt syntax.ATURI, subscriberDid string) error {
	_, err := e.Exec(`
		insert or ignore into discussion_subscriptions (discussion_at, subscriber_did)
		values (?, ?)
	`, discussionAt, subscriberDid)
	return err
}

// UnsubscribeFromDiscussion removes a subscription
func UnsubscribeFromDiscussion(e Execer, discussionAt syntax.ATURI, subscriberDid string) error {
	_, err := e.Exec(`
		delete from discussion_subscriptions
		where discussion_at = ? and subscriber_did = ?
	`, discussionAt, subscriberDid)
	return err
}

// GetDiscussionSubscribers returns all subscribers for a discussion
func GetDiscussionSubscribers(e Execer, discussionAt syntax.ATURI) ([]string, error) {
	rows, err := e.Query(`
		select subscriber_did from discussion_subscriptions
		where discussion_at = ?
	`, discussionAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscribers []string
	for rows.Next() {
		var did string
		if err := rows.Scan(&did); err != nil {
			return nil, err
		}
		subscribers = append(subscribers, did)
	}

	return subscribers, rows.Err()
}
