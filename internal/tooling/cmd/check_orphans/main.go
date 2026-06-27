package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func goList(args ...string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "go", append([]string{"list"}, args...)...) //nolint:gosec // args are hardcoded at every call site
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list %v: %w", args, err)
	}
	var result []string
	for line := range strings.SplitSeq(out.String(), "\n") {
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func main() {
	skipFlag := flag.String("skip", "", "comma-separated list of package path prefixes to ignore")
	tagsFlag := flag.String("tags", "", "comma-separated list of extra build tags to include when scanning test imports")
	flag.Parse()

	var skipPrefixes []string
	if *skipFlag != "" {
		for p := range strings.SplitSeq(*skipFlag, ",") {
			if p = strings.TrimSpace(p); p != "" {
				skipPrefixes = append(skipPrefixes, p)
			}
		}
	}

	var extraTagSets [][]string
	if *tagsFlag != "" {
		for raw := range strings.SplitSeq(*tagsFlag, ",") {
			if tag := strings.TrimSpace(raw); tag != "" {
				extraTagSets = append(extraTagSets, []string{"-tags", tag})
			}
		}
	}

	modLines, err := goList("-m", "-f", "{{.Path}}")
	if err != nil || len(modLines) == 0 {
		fmt.Fprintf(os.Stderr, "check-orphans: go list -m: %v\n", err)
		os.Exit(1)
	}
	mod := modLines[0]

	// -e prevents a non-zero exit for packages whose build constraints exclude all
	// files (e.g. e2e_test.go with //go:build e2e). The .Error filter drops them.
	allPkgs, err := goList("-e", "-f", "{{if not .Error}}{{.ImportPath}}{{end}}", "./...")
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-orphans: go list ./...: %v\n", err)
		os.Exit(1)
	}

	// generate-only packages (sole file is generate.go) exist only to host
	// //go:generate directives; their real output lives in sub-packages.
	// They are never imported and should not be flagged as orphans.
	genOnlyRaw, err := goList(
		"-e", "-f", `{{if and (not .Error) (eq (len .GoFiles) 1) (eq (index .GoFiles 0) "generate.go")}}{{.ImportPath}}{{end}}`,
		"./...",
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-orphans: go list gen-only: %v\n", err)
		os.Exit(1)
	}
	generateOnly := make(map[string]bool, len(genOnlyRaw))
	for _, p := range genOnlyRaw {
		if p != "" {
			generateOnly[p] = true
		}
	}

	reachable := make(map[string]bool)

	// Reachable from production: transitive deps of all main packages.
	mainPkgsRaw, err := goList("-e", "-f", "{{if and (not .Error) (eq .Name \"main\")}}{{.ImportPath}}{{end}}", "./...")
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-orphans: go list mains: %v\n", err)
		os.Exit(1)
	}
	var mainPkgs []string
	for _, p := range mainPkgsRaw {
		if p != "" {
			mainPkgs = append(mainPkgs, p)
		}
	}
	if len(mainPkgs) > 0 {
		fromMains, err := goList(append([]string{"-deps"}, mainPkgs...)...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "check-orphans: go list -deps mains: %v\n", err)
			os.Exit(1)
		}
		for _, p := range fromMains {
			reachable[p] = true
		}
	}

	// Reachable from tests: collect direct module-level test imports (TestImports
	// + XTestImports), then take their transitive deps. This is correct where
	// `go list -test -deps ./...` is not — the latter includes every package as
	// its own test dep, making the set diff always empty.
	//
	// The scan is repeated for each extra build tag set so that packages only
	// imported from tagged test files (e.g. //go:build integration) are not
	// falsely flagged as orphans.
	seen := make(map[string]bool)
	var testImports []string

	collectTestImports := func(extraArgs []string) {
		args := append(
			[]string{"-e", "-f", "{{if not .Error}}{{range .TestImports}}{{println .}}{{end}}{{range .XTestImports}}{{println .}}{{end}}{{end}}"},
			append(extraArgs, "./...")...,
		)
		raw, err := goList(args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "check-orphans: go list test imports %v: %v\n", extraArgs, err)
			os.Exit(1)
		}
		for _, p := range raw {
			if p != "" && !seen[p] && (strings.HasPrefix(p, mod+"/") || p == mod) {
				seen[p] = true
				testImports = append(testImports, p)
			}
		}
	}

	collectTestImports(nil)
	for _, tagArgs := range extraTagSets {
		collectTestImports(tagArgs)
	}

	if len(testImports) > 0 {
		fromTests, err := goList(append([]string{"-deps"}, testImports...)...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "check-orphans: go list -deps test imports: %v\n", err)
			os.Exit(1)
		}
		for _, p := range fromTests {
			reachable[p] = true
		}
	}

	var orphans []string
	for _, p := range allPkgs {
		if reachable[p] || generateOnly[p] {
			continue
		}
		skipped := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(p, prefix) {
				skipped = true
				break
			}
		}
		if !skipped {
			orphans = append(orphans, p)
		}
	}

	if len(orphans) > 0 {
		for _, o := range orphans {
			fmt.Fprintf(os.Stderr, "check-orphans: %s\n", o)
		}
		fmt.Fprintf(os.Stderr, "check-orphans: %d orphaned package(s) found\n", len(orphans))
		os.Exit(1)
	}

	fmt.Println("check-orphans: all packages are reachable.")
}
