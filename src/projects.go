package main

import (
	"io/fs"
	"os"
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
		if (strings.HasPrefix(name, ".") && name != ".git") || strings.HasPrefix(name, "ncs") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if name == ".git" {
			parent := filepath.Dir(path)
			if !isSubmoduleGitFile(path, d) {
				projects = append(projects, entry{kind: "project", name: filepath.Base(parent), path: parent})
			}
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

// Submodules use a .git file pointing into the parent's .git/modules tree.
// Linked worktrees also use .git files, but point into .git/worktrees and stay
// selectable as projects.
func isSubmoduleGitFile(path string, d fs.DirEntry) bool {
	if d.IsDir() {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	gitDir, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
	if !ok {
		return false
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(filepath.Dir(path), gitDir)
	}
	parts := strings.Split(filepath.Clean(gitDir), string(filepath.Separator))
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == ".git" && parts[i+1] == "modules" {
			return true
		}
	}
	return false
}
