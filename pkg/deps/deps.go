package deps

import (
	// "encoding/json" // Removed as FindDependents returns data directly
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/mod/modfile"
)

// getModuleCanonicalPath reads a go.mod file and returns the module's canonical path.
func getModuleCanonicalPath(goModPath string) (string, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", goModPath, err)
	}
	modFile, err := modfile.Parse("go.mod", content, nil)
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
		goModPath := filepath.Join(moduleDirPath, "go.mod")

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
	modFile, err := modfile.Parse("go.mod", content, nil)
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

// FindDependents recursively finds all modules in the workspace that depend on the initial module.
// It takes the path to the initial module's directory.
// It returns a sorted list of simple names of dependent modules, a list of log messages, and an error if a fatal error occurs.
func FindDependents(initialModuleDir string) ([]string, []string, error) {
	var logs []string

	// 1. Determine initial module from the provided path
	initialModuleName := filepath.Base(initialModuleDir)

	initialModuleGoModPath := filepath.Join(initialModuleDir, "go.mod")
	initialModuleCanonicalPath, err := getModuleCanonicalPath(initialModuleGoModPath)
	if err != nil {
		// Original code had multiple Fprintf calls for this error. Consolidating.
		newErr := fmt.Errorf("getting canonical path for initial module '%s' in %s: %w. Ensure it's a Go module root", initialModuleName, initialModuleDir, err)
		logs = append(logs, newErr.Error())
		return nil, logs, newErr
	}
	logs = append(logs, fmt.Sprintf("Searching for dependents of module: %s (%s)", initialModuleName, initialModuleCanonicalPath))

	// 2. Determine workspace root (parent of the initial module directory) and find all modules
	workspaceRoot := filepath.Dir(initialModuleDir)

	allModulesDirToCanonical, canonicalToSimpleName, err := findAllModules(workspaceRoot)
	if err != nil {
		newErr := fmt.Errorf("finding all modules in workspace %s: %w", workspaceRoot, err)
		logs = append(logs, newErr.Error())
		return nil, logs, newErr
	}
	if len(allModulesDirToCanonical) == 0 {
		newErr := fmt.Errorf("no Go modules found in the workspace root: %s", workspaceRoot)
		logs = append(logs, newErr.Error())
		return nil, logs, newErr
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
			otherModuleGoModPath := filepath.Join(otherModuleDir, "go.mod")
			isDependent, err := hasDirectDependency(otherModuleGoModPath, currentModuleToFindDependenciesFor)
			if err != nil {
				logs = append(logs, fmt.Sprintf("Warning: error checking if module at %s depends on %s: %v", otherModuleDir, currentModuleToFindDependenciesFor, err))
				continue // Continue with other modules
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

	// 4. Output results preparation
	var resultNames []string
	if len(dependentsFoundCanonicalPaths) > 0 {
		for depCanonicalPath := range dependentsFoundCanonicalPaths {
			simpleName, ok := canonicalToSimpleName[depCanonicalPath]
			if ok {
				resultNames = append(resultNames, simpleName)
			} else {
				// This case should ideally not be reached if findAllModules is comprehensive
				logs = append(logs, fmt.Sprintf("Warning: simple name unknown for canonical path %s", depCanonicalPath))
				resultNames = append(resultNames, depCanonicalPath+" (simple name unknown)")
			}
		}
		sort.Strings(resultNames)
	}
	return resultNames, logs, nil
}
