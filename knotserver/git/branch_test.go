package git

import (
	"path/filepath"
	"slices"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"tangled.org/core/sets"
)

type BranchSuite struct {
	suite.Suite
	*RepoSuite
}

func TestBranchSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(BranchSuite))
}

func (s *BranchSuite) SetupTest() {
	s.RepoSuite = NewRepoSuite(s.T())
}

func (s *BranchSuite) TearDownTest() {
	s.RepoSuite.cleanup()
}

func (s *BranchSuite) setupRepoWithBranches() {
	s.init()

	// get the initial commit on master
	head, err := s.repo.r.Head()
	require.NoError(s.T(), err)
	initialCommit := head.Hash()

	// create multiple branches with commits
	// branch-1
	s.createBranch("branch-1", initialCommit)
	s.checkoutBranch("branch-1")
	_ = s.commitFile("file1.txt", "content 1", "Add file1 on branch-1")

	// branch-2
	s.createBranch("branch-2", initialCommit)
	s.checkoutBranch("branch-2")
	_ = s.commitFile("file2.txt", "content 2", "Add file2 on branch-2")

	// branch-3
	s.createBranch("branch-3", initialCommit)
	s.checkoutBranch("branch-3")
	_ = s.commitFile("file3.txt", "content 3", "Add file3 on branch-3")

	// branch-4
	s.createBranch("branch-4", initialCommit)
	s.checkoutBranch("branch-4")
	s.commitFile("file4.txt", "content 4", "Add file4 on branch-4")

	// back to master and make a commit
	s.checkoutBranch("master")
	s.commitFile("master-file.txt", "master content", "Add file on master")

	// verify we have multiple branches
	refs, err := s.repo.r.References()
	require.NoError(s.T(), err)

	branchCount := 0
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsBranch() {
			branchCount++
		}
		return nil
	})
	require.NoError(s.T(), err)

	// we should have 5 branches: master, branch-1, branch-2, branch-3, branch-4
	assert.Equal(s.T(), 5, branchCount, "expected 5 branches")
}

func (s *BranchSuite) TestBranches_All() {
	s.setupRepoWithBranches()

	branches, err := s.repo.Branches(&BranchesOptions{})
	require.NoError(s.T(), err)

	assert.Len(s.T(), branches, 5, "expected 5 branches")

	expectedBranches := sets.Collect(slices.Values([]string{
		"master",
		"branch-1",
		"branch-2",
		"branch-3",
		"branch-4",
	}))

	for _, branch := range branches {
		assert.True(s.T(), expectedBranches.Contains(branch.Reference.Name),
			"unexpected branch: %s", branch.Reference.Name)
		assert.NotEmpty(s.T(), branch.Reference.Hash, "branch hash should not be empty")
		assert.NotNil(s.T(), branch.Commit, "branch commit should not be nil")
	}
}

func (s *BranchSuite) TestBranches_WithLimit() {
	s.setupRepoWithBranches()

	tests := []struct {
		name          string
		limit         int
		expectedCount int
	}{
		{
			name:          "limit 1",
			limit:         1,
			expectedCount: 1,
		},
		{
			name:          "limit 2",
			limit:         2,
			expectedCount: 2,
		},
		{
			name:          "limit 3",
			limit:         3,
			expectedCount: 3,
		},
		{
			name:          "limit 10 (more than available)",
			limit:         10,
			expectedCount: 5,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			branches, err := s.repo.Branches(&BranchesOptions{
				Limit: tt.limit,
			})
			require.NoError(s.T(), err)
			assert.Len(s.T(), branches, tt.expectedCount, "expected %d branches", tt.expectedCount)
		})
	}
}

func (s *BranchSuite) TestBranches_WithOffset() {
	s.setupRepoWithBranches()

	tests := []struct {
		name          string
		offset        int
		expectedCount int
	}{
		{
			name:          "offset 0",
			offset:        0,
			expectedCount: 5,
		},
		{
			name:          "offset 1",
			offset:        1,
			expectedCount: 4,
		},
		{
			name:          "offset 2",
			offset:        2,
			expectedCount: 3,
		},
		{
			name:          "offset 4",
			offset:        4,
			expectedCount: 1,
		},
		{
			name:          "offset 5 (all skipped)",
			offset:        5,
			expectedCount: 0,
		},
		{
			name:          "offset 10 (more than available)",
			offset:        10,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			branches, err := s.repo.Branches(&BranchesOptions{
				Offset: tt.offset,
			})
			require.NoError(s.T(), err)
			assert.Len(s.T(), branches, tt.expectedCount, "expected %d branches", tt.expectedCount)
		})
	}
}

func (s *BranchSuite) TestBranches_WithLimitAndOffset() {
	s.setupRepoWithBranches()

	tests := []struct {
		name          string
		limit         int
		offset        int
		expectedCount int
	}{
		{
			name:          "limit 2, offset 0",
			limit:         2,
			offset:        0,
			expectedCount: 2,
		},
		{
			name:          "limit 2, offset 1",
			limit:         2,
			offset:        1,
			expectedCount: 2,
		},
		{
			name:          "limit 2, offset 3",
			limit:         2,
			offset:        3,
			expectedCount: 2,
		},
		{
			name:          "limit 2, offset 4",
			limit:         2,
			offset:        4,
			expectedCount: 1,
		},
		{
			name:          "limit 3, offset 2",
			limit:         3,
			offset:        2,
			expectedCount: 3,
		},
		{
			name:          "limit 10, offset 3",
			limit:         10,
			offset:        3,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			branches, err := s.repo.Branches(&BranchesOptions{
				Limit:  tt.limit,
				Offset: tt.offset,
			})
			require.NoError(s.T(), err)
			assert.Len(s.T(), branches, tt.expectedCount, "expected %d branches", tt.expectedCount)
		})
	}
}

func (s *BranchSuite) TestBranches_EmptyRepo() {
	repoPath := filepath.Join(s.tempDir, "empty-repo")

	_, err := gogit.PlainInit(repoPath, false)
	require.NoError(s.T(), err)

	gitRepo, err := PlainOpen(repoPath)
	require.NoError(s.T(), err)

	branches, err := gitRepo.Branches(&BranchesOptions{})
	require.NoError(s.T(), err)

	if branches != nil {
		assert.Empty(s.T(), branches, "expected no branches in empty repo")
	}
}

func (s *BranchSuite) TestBranches_Pagination() {
	s.setupRepoWithBranches()

	allBranches, err := s.repo.Branches(&BranchesOptions{})
	require.NoError(s.T(), err)
	assert.Len(s.T(), allBranches, 5, "expected 5 branches")

	pageSize := 2
	var paginatedBranches []string

	for offset := 0; offset < len(allBranches); offset += pageSize {
		branches, err := s.repo.Branches(&BranchesOptions{
			Limit:  pageSize,
			Offset: offset,
		})
		require.NoError(s.T(), err)
		for _, branch := range branches {
			paginatedBranches = append(paginatedBranches, branch.Reference.Name)
		}
	}

	assert.Len(s.T(), paginatedBranches, len(allBranches), "pagination should return all branches")

	// create sets to verify all branches are present
	allBranchNames := sets.New[string]()
	for _, branch := range allBranches {
		allBranchNames.Insert(branch.Reference.Name)
	}

	paginatedBranchNames := sets.New[string]()
	for _, name := range paginatedBranches {
		paginatedBranchNames.Insert(name)
	}

	assert.EqualValues(s.T(), allBranchNames, paginatedBranchNames,
		"pagination should return the same set of branches")
}

func (s *BranchSuite) TestBranches_VerifyBranchFields() {
	s.setupRepoWithBranches()

	branches, err := s.repo.Branches(&BranchesOptions{})
	require.NoError(s.T(), err)

	found := false
	for i := range branches {
		if branches[i].Reference.Name == "master" {
			found = true
			assert.Equal(s.T(), "master", branches[i].Reference.Name)
			assert.NotEmpty(s.T(), branches[i].Reference.Hash)
			assert.NotNil(s.T(), branches[i].Commit)
			assert.NotEmpty(s.T(), branches[i].Commit.Author.Name)
			assert.NotEmpty(s.T(), branches[i].Commit.Author.Email)
			assert.False(s.T(), branches[i].Commit.Hash.IsZero())
			break
		}
	}

	assert.True(s.T(), found, "master branch not found")
}

func (s *BranchSuite) TestBranches_NilOptions() {
	s.setupRepoWithBranches()

	branches, err := s.repo.Branches(nil)
	require.NoError(s.T(), err)
	assert.Len(s.T(), branches, 5, "nil options should return all branches")
}

func (s *BranchSuite) TestBranches_ZeroLimitAndOffset() {
	s.setupRepoWithBranches()

	branches, err := s.repo.Branches(&BranchesOptions{
		Limit:  0,
		Offset: 0,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), branches, 5, "zero limit should return all branches")
}
