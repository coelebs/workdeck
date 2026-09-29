package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// Match the old find ~/ -maxdepth 4 -name .git behavior, including .git files
// in worktrees. Avoid searching inside the configurable tries directory.
func findProjects(home, triesDir string) ([]entry, error) {
	var projects []entry
	err := filepath.WalkDir(home, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == home {
				return walkErr
			}
			return nil
		}
		if path == triesDir {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		depth := strings.Count(rel, string(filepath.Separator)) + 1
		if d.IsDir() && depth >= 4 && d.Name() != ".git" {
			return filepath.SkipDir
		}
		name := d.Name()
		if strings.HasPrefix(name, ".trash") || strings.HasPrefix(name, "ncs") || strings.HasPrefix(name, ".cache") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if name == ".git" {
			parent := filepath.Dir(path)
			projects = append(projects, entry{kind: "project", name: filepath.Base(parent), path: parent})
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].path > projects[j].path })
	return projects, nil
}
