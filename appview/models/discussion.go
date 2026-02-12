package models

import (
	"fmt"
	"sort"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
)

// DiscussionState represents the state of a discussion
type DiscussionState int

const (
	DiscussionClosed DiscussionState = iota
	DiscussionOpen
	DiscussionMerged
)

func (s DiscussionState) String() string {
	switch s {
	case DiscussionOpen:
		return "open"
	case DiscussionMerged:
		return "merged"
	case DiscussionClosed:
		return "closed"
	default:
		return "closed"
	}
}

func (s DiscussionState) IsOpen() bool   { return s == DiscussionOpen }
func (s DiscussionState) IsMerged() bool { return s == DiscussionMerged }
func (s DiscussionState) IsClosed() bool { return s == DiscussionClosed }

// Discussion represents a discussion in a Pijul repository
// Anyone can add patches to a discussion
type Discussion struct {
	// ids
	Id           int64
	Did          string
	Rkey         string
	RepoAt       syntax.ATURI
	DiscussionId int

	// content
	Title         string
	Body          string
	TargetChannel string
	State         DiscussionState

	// meta
	Created time.Time
	Edited  *time.Time

	// populated on query
	Patches  []*DiscussionPatch
	Comments []DiscussionComment
	Labels   LabelState
	Repo     *Repo
}

const DiscussionNSID = "sh.tangled.repo.discussion"

func (d *Discussion) AtUri() syntax.ATURI {
	return syntax.ATURI(fmt.Sprintf("at://%s/%s/%s", d.Did, DiscussionNSID, d.Rkey))
}

// ActivePatches returns only the patches that haven't been removed
func (d *Discussion) ActivePatches() []*DiscussionPatch {
	var active []*DiscussionPatch
	for _, p := range d.Patches {
		if p.IsActive() {
			active = append(active, p)
		}
	}
	return active
}

// Participants returns all DIDs that have participated in this discussion
// (creator + patch pushers + commenters)
func (d *Discussion) Participants() []string {
	participantSet := make(map[string]struct{})
	participants := []string{}

	addParticipant := func(did string) {
		if _, exists := participantSet[did]; !exists {
			participantSet[did] = struct{}{}
			participants = append(participants, did)
		}
	}

	// Discussion creator
	addParticipant(d.Did)

	// Patch pushers
	for _, p := range d.Patches {
		addParticipant(p.PushedByDid)
	}

	// Commenters
	for _, c := range d.Comments {
		addParticipant(c.Did)
	}

	return participants
}

// CommentList returns a threaded comment list
func (d *Discussion) CommentList() []DiscussionCommentListItem {
	toplevel := make(map[string]*DiscussionCommentListItem)
	var replies []*DiscussionComment

	for i := range d.Comments {
		comment := &d.Comments[i]
		if comment.IsTopLevel() {
			toplevel[comment.AtUri().String()] = &DiscussionCommentListItem{
				Self: comment,
			}
		} else {
			replies = append(replies, comment)
		}
	}

	for _, r := range replies {
		parentAt := *r.ReplyTo
		if parent, exists := toplevel[parentAt]; exists {
			parent.Replies = append(parent.Replies, r)
		}
	}

	var listing []DiscussionCommentListItem
	for _, v := range toplevel {
		listing = append(listing, *v)
	}

	// Sort by creation time
	sortFunc := func(a, b *DiscussionComment) bool {
		return a.Created.Before(b.Created)
	}
	sort.Slice(listing, func(i, j int) bool {
		return sortFunc(listing[i].Self, listing[j].Self)
	})
	for _, r := range listing {
		sort.Slice(r.Replies, func(i, j int) bool {
			return sortFunc(r.Replies[i], r.Replies[j])
		})
	}

	return listing
}

// TotalComments returns the total number of comments
func (d *Discussion) TotalComments() int {
	return len(d.Comments)
}

// DiscussionPatch represents a patch added to a discussion
// Key difference from PullSubmission: it has pushed_by_did
type DiscussionPatch struct {
	Id           int64
	DiscussionAt syntax.ATURI
	PushedByDid  string
	PatchHash    string
	Patch        string
	Added        time.Time
	Removed      *time.Time
}

// IsActive returns true if the patch hasn't been removed
func (p *DiscussionPatch) IsActive() bool {
	return p.Removed == nil
}

// CanRemove checks if the given user can remove this patch
// A patch can be removed by:
// 1. The person who pushed it
// 2. Someone with edit permissions on the repo
func (p *DiscussionPatch) CanRemove(userDid string, hasEditPerm bool) bool {
	return p.PushedByDid == userDid || hasEditPerm
}

// DiscussionComment represents a comment on a discussion
type DiscussionComment struct {
	Id           int64
	Did          string
	Rkey         string
	DiscussionAt string
	ReplyTo      *string
	Body         string
	Created      time.Time
	Edited       *time.Time
	Deleted      *time.Time
}

const DiscussionCommentNSID = "sh.tangled.repo.discussion.comment"

func (c *DiscussionComment) AtUri() syntax.ATURI {
	return syntax.ATURI(fmt.Sprintf("at://%s/%s/%s", c.Did, DiscussionCommentNSID, c.Rkey))
}

func (c *DiscussionComment) IsTopLevel() bool {
	return c.ReplyTo == nil
}

func (c *DiscussionComment) IsReply() bool {
	return c.ReplyTo != nil
}

// DiscussionCommentListItem represents a top-level comment with its replies
type DiscussionCommentListItem struct {
	Self    *DiscussionComment
	Replies []*DiscussionComment
}

// Participants returns all DIDs that participated in this comment thread
func (item *DiscussionCommentListItem) Participants() []syntax.DID {
	participantSet := make(map[syntax.DID]struct{})
	participants := []syntax.DID{}

	addParticipant := func(did syntax.DID) {
		if _, exists := participantSet[did]; !exists {
			participantSet[did] = struct{}{}
			participants = append(participants, did)
		}
	}

	addParticipant(syntax.DID(item.Self.Did))

	for _, c := range item.Replies {
		addParticipant(syntax.DID(c.Did))
	}

	return participants
}

// DiscussionCount holds counts for different discussion states
type DiscussionCount struct {
	Open   int
	Merged int
	Closed int
}
