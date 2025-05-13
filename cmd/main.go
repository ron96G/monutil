package main

import (
	"flag"

	"github.com/ron96g/go-mono-util/pkg/diff"
)

var (
	baseCommitSha string
	headCommitSha string
	targetModule  string
	verbose       bool
	debug         bool
)

func init() {
	flag.StringVar(&targetModule, "module", ".", "Target module to check dependencies for")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&debug, "debug", false, "Enable debug output")
	flag.StringVar(&baseCommitSha, "base", "HEAD~1", "Base commit SHA for diff")
	flag.StringVar(&headCommitSha, "head", "HEAD", "Head commit SHA for diff")
}

func main() {
	flag.Parse()

	// dependents, _, err := deps.FindDependents(targetModule)
	// if err != nil {
	// 	panic(err)
	// }

	// for _, dependent := range dependents {
	// 	println(dependent)
	// }

	_, err := diff.FindChangedModules(baseCommitSha, headCommitSha)
	if err != nil {
		panic(err)
	}
}
