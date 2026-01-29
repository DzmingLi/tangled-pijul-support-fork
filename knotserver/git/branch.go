package git

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"tangled.org/core/types"
)

type BranchesOptions struct {
	Limit  int
	Offset int
}

func (g *GitRepo) Branches(opts *BranchesOptions) ([]types.Branch, error) {
	if opts == nil {
		opts = &BranchesOptions{}
	}

	fields := []string{
		"refname:short",
		"objectname",
		"authorname",
		"authoremail",
		"authordate:unix",
		"committername",
		"committeremail",
		"committerdate:unix",
		"tree",
		"parent",
		"contents",
	}

	var outFormat strings.Builder
	outFormat.WriteString("--format=")
	for i, f := range fields {
		if i != 0 {
			outFormat.WriteString(fieldSeparator)
		}
		fmt.Fprintf(&outFormat, "%%(%s)", f)
	}
	outFormat.WriteString("")
	outFormat.WriteString(recordSeparator)

	args := []string{outFormat.String(), "--sort=-creatordate"}

	// only add the count if the limit is a non-zero value,
	// if it is zero, get as many tags as we can
	if opts.Limit > 0 {
		args = append(args, fmt.Sprintf("--count=%d", opts.Offset+opts.Limit))
	}

	args = append(args, "refs/heads")

	output, err := g.forEachRef(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	records := strings.Split(strings.TrimSpace(string(output)), recordSeparator)
	if len(records) == 1 && records[0] == "" {
		return nil, nil
	}

	startIdx := opts.Offset
	if startIdx >= len(records) {
		return nil, nil
	}

	endIdx := len(records)
	if opts.Limit > 0 {
		endIdx = min(startIdx+opts.Limit, len(records))
	}

	records = records[startIdx:endIdx]
	branches := make([]types.Branch, 0, len(records))

	// ignore errors here
	defaultBranch, _ := g.FindMainBranch()

	for _, line := range records {
		parts := strings.SplitN(strings.TrimSpace(line), fieldSeparator, len(fields))
		if len(parts) < 6 {
			continue
		}

		branchName := parts[0]
		commitHash := plumbing.NewHash(parts[1])
		authorName := parts[2]
		authorEmail := strings.TrimSuffix(strings.TrimPrefix(parts[3], "<"), ">")
		authorDate := parts[4]
		committerName := parts[5]
		committerEmail := strings.TrimSuffix(strings.TrimPrefix(parts[6], "<"), ">")
		committerDate := parts[7]
		treeHash := plumbing.NewHash(parts[8])
		parentHash := plumbing.NewHash(parts[9])
		message := parts[10]

		// parse creation time
		var authoredAt, committedAt time.Time
		if unix, err := strconv.ParseInt(authorDate, 10, 64); err == nil {
			authoredAt = time.Unix(unix, 0)
		}
		if unix, err := strconv.ParseInt(committerDate, 10, 64); err == nil {
			committedAt = time.Unix(unix, 0)
		}

		branch := types.Branch{
			IsDefault: branchName == defaultBranch,
			Reference: types.Reference{
				Name: branchName,
				Hash: commitHash.String(),
			},
			Commit: &object.Commit{
				Hash: commitHash,
				Author: object.Signature{
					Name:  authorName,
					Email: authorEmail,
					When:  authoredAt,
				},
				Committer: object.Signature{
					Name:  committerName,
					Email: committerEmail,
					When:  committedAt,
				},
				TreeHash:     treeHash,
				ParentHashes: []plumbing.Hash{parentHash},
				Message:      message,
			},
		}

		branches = append(branches, branch)
	}

	slices.Reverse(branches)
	return branches, nil
}

func (g *GitRepo) Branch(name string) (*plumbing.Reference, error) {
	ref, err := g.r.Reference(plumbing.NewBranchReferenceName(name), false)
	if err != nil {
		return nil, fmt.Errorf("branch: %w", err)
	}

	if !ref.Name().IsBranch() {
		return nil, fmt.Errorf("branch: %s is not a branch", ref.Name())
	}

	return ref, nil
}

func (g *GitRepo) DeleteBranch(branch string) error {
	ref := plumbing.NewBranchReferenceName(branch)
	return g.r.Storer.RemoveReference(ref)
}
