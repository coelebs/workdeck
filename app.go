package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type entry struct {
	kind string
	name string
	path string
}

// A command's interactive mode inherits the terminal. Otherwise stdout is captured.
type commandRunner func(name string, args []string, input string, interactive bool) (string, error)

type app struct {
	home       string
	run        commandRunner
	insideTmux bool
}

func (a app) execute(args []string) error {
	if len(args) > 1 {
		return errors.New("usage: tmux-sessionizer [directory]")
	}

	var selected entry
	if len(args) == 1 {
		path, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("not a directory: %s", path)
		}
		selected = entry{kind: "project", name: filepath.Base(path), path: path}
	} else {
		projects, err := findProjects(a.home, "")
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			return nil
		}
		choice, ok, err := a.pick(projects)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		selected = choice
	}
	return a.openSession(selected)
}

func (a app) pick(entries []entry) (entry, bool, error) {
	var input strings.Builder
	for i, item := range entries {
		// fzf returns the original input line. Only the numeric ID is used to
		// resolve a selection, so display escaping cannot alter a real path.
		fmt.Fprintf(&input, "%d\t%s\t%s\t%s\n", i, item.kind,
			cleanDisplay(item.name), cleanDisplay(item.path))
	}

	output, err := a.run("fzf", []string{
		"--delimiter=\t", "--with-nth=2..", "--nth=2", "--tiebreak=index",
	}, input.String(), false)
	if err != nil {
		var exit *execExitError
		if errors.As(err, &exit) && (exit.code == 1 || exit.code == 130) {
			return entry{}, false, nil
		}
		return entry{}, false, fmt.Errorf("fzf: %w", err)
	}
	line := strings.TrimSuffix(output, "\n")
	id, _, ok := strings.Cut(line, "\t")
	if !ok {
		return entry{}, false, fmt.Errorf("invalid fzf selection: %q", output)
	}
	i, err := strconv.Atoi(id)
	if err != nil || i < 0 || i >= len(entries) {
		return entry{}, false, fmt.Errorf("invalid fzf selection: %q", output)
	}
	return entries[i], true, nil
}

func cleanDisplay(s string) string {
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
}
