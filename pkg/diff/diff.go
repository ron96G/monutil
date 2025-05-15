package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pkg/errors"
)

// If the commit SHA is all zeros, it means the first commit in the repo.
// This is a special case that we need to handle when checking for changes.
// In this case, we treat all files as changed, since there is no base to diff against.
const firstCommitSha = "0000000000000000000000000000000000000000"

func FindChangedPaths(beforeCommitSha, headCommitSha string, depth int, filePattern *regexp.Regexp) ([]string, error) {
	gitRepo, err := git.PlainOpen(".")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open git repository")
	}

	var baseTree *object.Tree
	if beforeCommitSha != "" && !strings.HasPrefix(beforeCommitSha, firstCommitSha) {
		baseHash := plumbing.NewHash(beforeCommitSha)
		// Get the commit object for the base reference
		baseCommit, err := gitRepo.CommitObject(baseHash)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get commit object for base reference")
		}
		baseTree, err = baseCommit.Tree()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get tree object for base commit")
		}
	}

	var headHash plumbing.Hash
	if headCommitSha != "" {
		headHash = plumbing.NewHash(headCommitSha)
	} else {
		// Get the HEAD reference
		headRef, err := gitRepo.Reference(plumbing.HEAD, true)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get HEAD reference")
		}
		headHash = headRef.Hash()
		fmt.Fprintf(os.Stderr, "HEAD hash: %s\n", headHash.String())
	}

	headCommit, err := gitRepo.CommitObject(headHash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get commit object for HEAD reference")
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get tree object for HEAD commit")
	}

	changes, err := object.DiffTree(baseTree, headTree)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get diff between trees")
	}

	return GetChangedPaths(changes, depth, filePattern)
}

func GetChangedPaths(changes object.Changes, maxDepth int, filePattern *regexp.Regexp) ([]string, error) {
	foundPaths := make(map[string]struct{})
	changedPaths := []string{}

	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get action for change")
		}

		if action.String() == "Delete" {
			continue
		}

		path := change.To.Name
		if !filePattern.MatchString(path) {
			fmt.Fprintf(os.Stderr, "Skipping %s\n", path)
			continue
		}

		// check for maxDepth
		parts := strings.Split(path, "/")
		if len(parts) > maxDepth {
			path = filepath.Join(parts[:maxDepth]...)
		}

		if _, ok := foundPaths[path]; ok {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, errors.Wrapf(err, "failed to stat path %s", path)
		}
		if !info.IsDir() {
			continue
		}

		foundPaths[path] = struct{}{}
		changedPaths = append(changedPaths, path)
	}

	return changedPaths, nil

}
