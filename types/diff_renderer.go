package types

import "tangled.org/core/appview/filetree"

type DiffRenderer interface {
	// list of file affected by these diffs
	ChangedFiles() []DiffFileRenderer

	// filetree
	FileTree() *filetree.FileTreeNode

	Stats() DiffStat
}

type DiffFileRenderer interface {
	// html ID for each file in the diff
	Id() string

	// produce a splitdiff
	Split() SplitDiff

	// stats for this single file
	Stats() DiffFileStat

	// old and new name of file
	Names() DiffFileName

	// whether this diff can be displayed,
	// returns a reason if not, and the empty string if it can
	CanRender() string
}
