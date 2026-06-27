package lintasyncapi

import (
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

// Violation describes a single address that does not satisfy the prefix convention.
type Violation struct {
	File           string
	Address        string
	RequiredPrefix string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s: address %q must start with %q", v.File, v.Address, v.RequiredPrefix+".")
}

// Linter validates that every channel address in an AsyncAPI spec tree
// follows the public.<module>.<event> / private.<module>.<event> convention.
type Linter struct {
	fsys fs.FS
}

// NewLinter returns a Linter rooted at fsys.
// fsys must represent the contracts/asyncapi directory:
// files under public/ must have addresses starting with "public.",
// files under private/ must have addresses starting with "private.".
func NewLinter(fsys fs.FS) Linter {
	return Linter{fsys: fsys}
}

// Lint walks the FS and returns all convention violations.
// An error is returned only for unexpected I/O or parse failures.
func (l Linter) Lint() ([]Violation, error) {
	var violations []Violation

	err := fs.WalkDir(l.fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "asyncapi.yaml" {
			return nil
		}

		prefix, ok := requiredPrefix(path)
		if !ok {
			return nil
		}

		vs, err := lintFile(l.fsys, path, prefix)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		violations = append(violations, vs...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking asyncapi directory: %w", err)
	}

	return violations, nil
}

// requiredPrefix returns the required address prefix for an asyncapi.yaml at
// the given path. Paths starting with "public/" require "public"; paths
// starting with "private/" require "private". Other paths are ignored.
func requiredPrefix(path string) (string, bool) {
	switch {
	case strings.HasPrefix(path, "public/"):
		return "public", true
	case strings.HasPrefix(path, "private/"):
		return "private", true
	default:
		return "", false
	}
}

type asyncAPISpec struct {
	Channels map[string]struct {
		Address string `yaml:"address"`
	} `yaml:"channels"`
}

func lintFile(fsys fs.FS, path, requiredPrefix string) ([]Violation, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var spec asyncAPISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	var violations []Violation
	for _, ch := range spec.Channels {
		if ch.Address == "" {
			continue
		}
		if !strings.HasPrefix(ch.Address, requiredPrefix+".") {
			violations = append(violations, Violation{
				File:           path,
				Address:        ch.Address,
				RequiredPrefix: requiredPrefix,
			})
		}
	}
	return violations, nil
}
