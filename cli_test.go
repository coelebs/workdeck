package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the built program with the real fzf filter and a fake tmux, so
// argument parsing, environment configuration and process I/O are covered.
func TestCLI(t *testing.T) {
	if _, err := exec.LookPath("fzf"); err != nil {
		t.Skip("fzf not installed")
	}
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(binDir, "tmux-sessionizer")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}
	home := filepath.Join(root, "home")
	tries := filepath.Join(home, "experiments")
	log := filepath.Join(root, "tmux.log")
	// A TTY is not needed by the stub. The real fzf runs in --filter mode.
	stub := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$TMUX_LOG\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "tmux"), []byte(stub), 0755); err != nil {
		t.Fatal(err)
	}
	baseEnv := append(os.Environ(),
		"HOME="+home,
		"TMUX_SESSIONIZER_TRIES_DIR="+tries,
		"TMUX_LOG="+log,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TMUX=",
	)
	run := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Env = env
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%q: %v: %s", args, err, output)
		}
	}
	run(baseEnv, "try", "new idea")
	children, err := os.ReadDir(tries)
	if err != nil || len(children) != 1 {
		t.Fatalf("try not created: %v, %v", children, err)
	}
	trial := filepath.Join(tries, children[0].Name())
	if !strings.HasSuffix(trial, "-new-idea") {
		t.Fatalf("wrong try name: %s", trial)
	}
	if _, err := os.Stat(filepath.Join(trial, ".git")); !os.IsNotExist(err) {
		t.Fatalf("try contains git metadata: %v", err)
	}

	// With one matching name, fzf --filter selects the try without a TTY.
	run(append(baseEnv, "FZF_DEFAULT_OPTS=--filter=new-idea"))
	content, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := "new-session -A -s try-" + children[0].Name() + " -c " + trial + "\n"
	if string(content) != want+want {
		t.Fatalf("wrong tmux calls: %q, want %q", content, want+want)
	}

	missingName := exec.Command(binary, "try")
	missingName.Env = baseEnv
	if err := missingName.Run(); err == nil {
		t.Fatal("missing try name was accepted")
	}
}

func TestFzfPrefersProjectForSimilarMatch(t *testing.T) {
	if _, err := exec.LookPath("fzf"); err != nil {
		t.Skip("fzf not installed")
	}
	cmd := exec.Command("fzf", "--filter=redis", "--delimiter=\t", "--with-nth=2..", "--nth=2", "--tiebreak=index")
	cmd.Stdin = strings.NewReader("0\tproject\tredis-server\t/projects/redis-server\n1\ttry\t2026-09-29-redis\t/tries/2026-09-29-redis\n")
	output, err := cmd.Output()
	if err != nil || !strings.HasPrefix(string(output), "0\tproject\tredis-server") {
		t.Fatalf("fzf ranking: %q, %v", output, err)
	}
}
