package deps

import (
	// "encoding/json" // Removed as FindDependents returns data directly
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"
	"golang.org/x/mod/modfile"
)

var gomod = "go.mod"

// getModuleCanonicalPath reads a go.mod file and returns the module's canonical path.
func getModuleCanonicalPath(goModPath string) (string, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", goModPath, err)
	}
	modFile, err := modfile.Parse(gomod, content, nil)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", goModPath, err)
	}
	return modFile.Module.Mod.Path, nil
}

// findAllModules scans the workspace root for directories containing go.mod files
// and returns a map of their directory paths to their canonical module paths,
// and a map of canonical module paths to their simple names (dir base names).
func findAllModules(workspaceRoot string) (map[string]string, map[string]string, error) {
	moduleDirToCanonicalPath := make(map[string]string)
	canonicalPathToSimpleName := make(map[string]string)

	entries, err := os.ReadDir(workspaceRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("reading workspace root %s: %w", workspaceRoot, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		moduleDirPath := filepath.Join(workspaceRoot, entry.Name())
		goModPath := filepath.Join(moduleDirPath, gomod)

		if stat, err := os.Stat(goModPath); err == nil && !stat.IsDir() { // Corrected syntax error here
			canonicalPath, err := getModuleCanonicalPath(goModPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not get canonical path for module in %s: %v\n", moduleDirPath, err) // This log remains as it's a non-fatal warning within this specific function
				continue
			}
			moduleDirToCanonicalPath[moduleDirPath] = canonicalPath
			canonicalPathToSimpleName[canonicalPath] = entry.Name()
		}
	}
	return moduleDirToCanonicalPath, canonicalPathToSimpleName, nil
}

// hasDirectDependency checks if the go.mod file at dependentGoModPath
// has a require directive for targetModuleCanonicalPath.
func hasDirectDependency(dependentGoModPath string, targetModuleCanonicalPath string) (bool, error) {
	content, err := os.ReadFile(dependentGoModPath)
	if err != nil {
		return false, fmt.Errorf("opening %s: %w", dependentGoModPath, err)
	}
	modFile, err := modfile.Parse(gomod, content, nil)
	if err != nil {
		return false, fmt.Errorf("parsing %s: %w", dependentGoModPath, err)
	}
	for _, req := range modFile.Require {
		if req.Mod.Path == targetModuleCanonicalPath {
			return true, nil
		}
	}
	return false, nil
}

type Dependent struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// FindDependents recursively finds all modules in the workspace that depend on the initial module.
// It takes the path to the initial module's directory and a boolean to indicate if the initial module itself should be included.
func FindDependents(initialModuleDir string, addSelf bool) ([]Dependent, error) {
	initialModuleGoModPath := filepath.Join(initialModuleDir, gomod)
	initialModuleCanonicalPath, err := getModuleCanonicalPath(initialModuleGoModPath)
	if err != nil {
		return nil, errors.Wrap(err, "getting initial module canonical path")
	}

	// 2. Determine workspace root (parent of the initial module directory) and find all modules
	workspaceRoot := filepath.Dir(initialModuleDir)

	allModulesDirToCanonical, _, err := findAllModules(workspaceRoot)
	if err != nil {
		return nil, errors.Wrap(err, "finding all modules")
	}
	if len(allModulesDirToCanonical) == 0 {
		return nil, fmt.Errorf("no modules found in workspace root %s", workspaceRoot)
	}

	// Create a reverse map from canonical path to module directory for easy lookup
	canonicalPathToModuleDir := make(map[string]string)
	for dir, canon := range allModulesDirToCanonical {
		canonicalPathToModuleDir[canon] = dir
	}

	// 3. Dependency Checking
	dependentsFoundCanonicalPaths := make(map[string]struct{})
	queue := []string{initialModuleCanonicalPath}
	visitedForQueue := make(map[string]struct{})
	visitedForQueue[initialModuleCanonicalPath] = struct{}{}

	head := 0
	for head < len(queue) {
		currentModuleToFindDependenciesFor := queue[head]
		head++

		for otherModuleDir, otherModuleCanonicalPath := range allModulesDirToCanonical {
			// A module cannot be its own dependent in this context for the final list.
			if otherModuleCanonicalPath == initialModuleCanonicalPath {
				continue
			}

			// Check if this 'otherModule' depends on 'currentModuleToFindDependenciesFor'
			otherModuleGoModPath := filepath.Join(otherModuleDir, gomod)
			isDependent, err := hasDirectDependency(otherModuleGoModPath, currentModuleToFindDependenciesFor)
			if err != nil {
				return nil, fmt.Errorf("checking dependency from %s to %s: %w", otherModuleGoModPath, currentModuleToFindDependenciesFor, err)
			}

			if isDependent {
				// If 'otherModule' depends on 'currentModuleToFindDependenciesFor',
				// and we haven't already recorded 'otherModule' as a dependent of the *initial* module:
				if _, alreadyFound := dependentsFoundCanonicalPaths[otherModuleCanonicalPath]; !alreadyFound {
					dependentsFoundCanonicalPaths[otherModuleCanonicalPath] = struct{}{}

					// Add this newly found dependent to the queue to check its dependents,
					// but only if it hasn't been queued/processed before.
					if _, visited := visitedForQueue[otherModuleCanonicalPath]; !visited {
						queue = append(queue, otherModuleCanonicalPath)
						visitedForQueue[otherModuleCanonicalPath] = struct{}{}
					}
				}
			}
		}
	}

	var results []Dependent
	if addSelf {
		results = append(results, Dependent{Name: initialModuleCanonicalPath, Path: initialModuleDir})
	}

	if len(dependentsFoundCanonicalPaths) > 0 {
		for depCanonicalPath := range dependentsFoundCanonicalPaths {
			moduleDir, ok := canonicalPathToModuleDir[depCanonicalPath]

			if ok {
				results = append(results, Dependent{Name: depCanonicalPath, Path: moduleDir})
			} else {
				return nil, fmt.Errorf("could not find module directory for canonical path %s", depCanonicalPath)
			}
		}
		// Sort results by name
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})
	}
	return results, nil
}
