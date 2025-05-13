package diff

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pkg/errors"
)

func FindChangedModules(headCommitSha, beforeCommitSha string) ([]string, error) {

	gitRepo, err := git.PlainOpen(".")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open git repository")
	}

	// Get the HEAD reference
	headRef, err := gitRepo.Reference(plumbing.ReferenceName(headCommitSha), true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get HEAD reference")
	}
	// Get the base reference
	baseRef, err := gitRepo.Reference(plumbing.ReferenceName(beforeCommitSha), true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get base reference")
	}

	// Get the commit object for the base reference
	baseCommit, err := gitRepo.CommitObject(baseRef.Hash())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get commit object for base reference")
	}
	// Get the commit object for the HEAD reference
	headCommit, err := gitRepo.CommitObject(headRef.Hash())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get commit object for HEAD reference")
	}

	baseTree, err := baseCommit.Tree()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get tree object for base commit")
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get tree object for HEAD commit")
	}

	changes, err := object.DiffTree(baseTree, headTree)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get diff between trees")
	}

	for _, change := range changes {
		fmt.Printf("Change: %s\n", change)
	}

	return nil, nil
}
