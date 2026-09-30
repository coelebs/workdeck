package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

func (a app) tryRoot() (string, error) {
	root := a.triesDir
	if root == "" {
		root = filepath.Join(a.home, "Projects", "tries")
	} else if root == "~" {
		root = a.home
	} else if strings.HasPrefix(root, "~/") {
		root = filepath.Join(a.home, root[2:])
	}
	return filepath.Abs(root)
}

func normalizeTryName(name string) (string, error) {
	for _, char := range name {
		if unicode.IsControl(char) {
			return "", errors.New("try name cannot contain control characters")
		}
	}
	name = strings.Join(strings.Fields(name), "-")
	if name == "" {
		return "", errors.New("try name cannot be empty")
	}
	for _, char := range name {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '-' && char != '_' && char != '.' {
			return "", fmt.Errorf("invalid try name %q: use letters, numbers, spaces, dots, dashes or underscores", name)
		}
	}
	if !unicode.IsLetter([]rune(name)[0]) && !unicode.IsDigit([]rune(name)[0]) {
		return "", fmt.Errorf("try name must start with a letter or number: %q", name)
	}
	return name, nil
}

func (a app) createTry(name string) (string, error) {
	name, err := normalizeTryName(name)
	if err != nil {
		return "", err
	}
	root, err := a.tryRoot()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	today := time.Now
	if a.today != nil {
		today = a.today
	}
	path := filepath.Join(root, today().Format("2006-01-02")+"-"+name)
	if err := os.Mkdir(path, 0755); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
		// Reusing a name reopens that day's directory. Do not follow an
		// existing symlink out of the configured tries directory.
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return "", statErr
		}
		if !info.IsDir() {
			return "", fmt.Errorf("try path exists but is not a directory: %s", path)
		}
	}
	return path, nil
}

func findTries(root string) ([]entry, error) {
	children, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tries []entry
	for _, child := range children {
		if child.IsDir() && !strings.HasPrefix(child.Name(), ".") {
			tries = append(tries, entry{kind: "try", name: child.Name(), path: filepath.Join(root, child.Name())})
		}
	}
	sort.Slice(tries, func(i, j int) bool { return tries[i].name > tries[j].name })
	return tries, nil
}
