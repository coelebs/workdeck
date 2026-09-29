package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type execExitError struct{ code int }

func (e *execExitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

func runCommand(name string, args []string, input string, interactive bool) (string, error) {
	cmd := exec.Command(name, args...)
	if interactive {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		return "", cmd.Run()
	}
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", &execExitError{code: exit.ExitCode()}
		}
	}
	return string(output), err
}

func projectSessionName(path string) string {
	// Preserve the naming used by the original shell script for existing
	// project sessions (including worktrees).
	parts := strings.Split(filepath.Base(filepath.Dir(path)), "-")
	worktreeName := parts[0] + "-"
	if len(parts) > 1 {
		worktreeName += parts[1]
	}
	runes := []rune(worktreeName)
	if len(runes) > 10 {
		worktreeName = string(runes[:10])
	}
	return worktreeName + "/" + strings.ReplaceAll(filepath.Base(path), ".", "_")
}

func (a app) openSession(item entry) error {
	name := projectSessionName(item.path)
	if item.kind == "try" {
		name = "try-" + item.name
	}
	if !a.insideTmux {
		_, err := a.run("tmux", []string{"new-session", "-A", "-s", name, "-c", item.path}, "", true)
		return err
	}

	_, err := a.run("tmux", []string{"has-session", "-t", "=" + name}, "", false)
	if err != nil {
		var exit *execExitError
		if !errors.As(err, &exit) || exit.code != 1 {
			return fmt.Errorf("checking tmux session %q: %w", name, err)
		}
		if _, err := a.run("tmux", []string{"new-session", "-d", "-s", name, "-c", item.path}, "", false); err != nil {
			return err
		}
	}
	_, err = a.run("tmux", []string{"switch-client", "-t", "=" + name}, "", false)
	return err
}
