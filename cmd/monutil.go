package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/ron96g/go-mono-util/pkg/deps"
	"github.com/ron96g/go-mono-util/pkg/diff"
)

var (
	baseCommitSha string
	headCommitSha string
	targetModule  string
	depth         int
	format        string
	filePattern   string
	verbose       bool
	debug         bool
)

func init() {
	flag.StringVar(&targetModule, "module", ".", "Target module to check dependencies for")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&debug, "debug", false, "Enable debug output")
	flag.StringVar(&baseCommitSha, "base", "", "Base commit SHA for diff")
	flag.StringVar(&headCommitSha, "head", "", "Head commit SHA for diff")
	flag.IntVar(&depth, "depth", 1, "Depth for diff")
	flag.StringVar(&format, "format", "json", "Output format (json or text)")
	flag.StringVar(&filePattern, "pattern", `^.*(\.go|go\.mod|go\.sum)$`, "File pattern to match")
}

func isModule(dir string) (bool, error) {
	info, err := os.Stat(filepath.Join(dir, "go.mod"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return false, nil
	}
	return true, nil
}

type FoundModule struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func ForEachChangedModule(changedDirs []string) ([]FoundModule, error) {
	allRelevantModules := make([]FoundModule, 0)

	for _, dir := range changedDirs {
		isMod, err := isModule(dir)
		if err != nil {
			return allRelevantModules, err
		}
		if isMod {
			dependents, err := deps.FindDependents(dir)
			if err != nil {
				return allRelevantModules, err
			}
			if len(dependents) == 0 {
				fmt.Fprintf(os.Stderr, "No dependents found for %s\n", dir)
				continue
			} else {
				fmt.Fprintf(os.Stderr, "Found %d dependents for %s\n", len(dependents), dir)
			}
			for _, dependent := range dependents {
				if slices.ContainsFunc(allRelevantModules, func(m FoundModule) bool {
					return m.Name == dependent.Name
				}) {
					continue
				}
				allRelevantModules = append(allRelevantModules, FoundModule{
					Name: dependent.Name,
					Path: dependent.Path,
				})
			}
		}
	}
	return allRelevantModules, nil
}

func main() {
	flag.Parse()

	changes, err := diff.FindChangedModules(baseCommitSha, headCommitSha, depth, regexp.MustCompile(filePattern))
	if err != nil {
		panic(err)
	}

	allRelevantModules, err := ForEachChangedModule(changes)
	if err != nil {
		panic(err)
	}

	if format == "json" {
		b, err := json.Marshal(allRelevantModules)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(b))
		return
	}

	for _, module := range allRelevantModules {
		println(module.Path)
	}
}
