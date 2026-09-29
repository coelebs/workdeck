package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTryName(t *testing.T) {
	for input, want := range map[string]string{
		"redis": "redis", "redis pool": "redis-pool", "  api   v2  ": "api-v2",
	} {
		got, err := normalizeTryName(input)
		if err != nil || got != want {
			t.Errorf("name %q: got %q, %v", input, got, err)
		}
	}
	for _, input := range []string{"", "  ", "../outside", "/tmp/outside", ".hidden", "a\nb", "foo/bar"} {
		if _, err := normalizeTryName(input); err == nil {
			t.Errorf("accepted invalid name %q", input)
		}
	}
}

func TestTryRoot(t *testing.T) {
	home := t.TempDir()
	for _, tc := range []struct{ setting, want string }{
		{"", filepath.Join(home, "Projects", "tries")},
		{"~/experiments", filepath.Join(home, "experiments")},
		{filepath.Join(home, "somewhere"), filepath.Join(home, "somewhere")},
	} {
		a := app{home: home, triesDir: tc.setting}
		got, err := a.tryRoot()
		if err != nil || got != tc.want {
			t.Fatalf("setting %q: got %q, %v, want %q", tc.setting, got, err, tc.want)
		}
	}
}

func TestCreateTryReusesDirectoryWithoutGit(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "experiments")
	a := app{home: home, triesDir: root, today: func() time.Time {
		return time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	}}
	path, err := a.createTry("redis pool")
	if err != nil || path != filepath.Join(root, "2026-09-29-redis-pool") {
		t.Fatalf("created %q: %v", path, err)
	}
	file := filepath.Join(path, "my-data")
	if err := os.WriteFile(file, []byte("kept"), 0644); err != nil {
		t.Fatal(err)
	}
	again, err := a.createTry("redis pool")
	if err != nil || again != path {
		t.Fatalf("reopened %q: %v", again, err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("lost existing try contents: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); !os.IsNotExist(err) {
		t.Fatalf("try initialized git: %v", err)
	}
	if err := os.Symlink(home, filepath.Join(root, "2026-09-29-symlink")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.createTry("symlink"); err == nil {
		t.Fatal("followed a symlink for an existing try")
	}
}

func TestTryMustHaveNameBeforeCreatingRoot(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "not-created")
	a := app{home: home, triesDir: root}
	for _, args := range [][]string{{"try"}, {"try", ""}, {"try", "one", "two"}, {"try", "../escape"}} {
		if err := a.execute(args); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("created root for invalid input: %v", err)
	}
}

func TestPickerIncludesTriesAfterProjects(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "Projects", "repo")
	root := filepath.Join(home, "src", "tries")
	trial := filepath.Join(root, "2026-09-29-test")
	for _, path := range []string{filepath.Join(project, ".git"), trial} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	var calls []call
	a := app{home: home, triesDir: root, run: func(name string, args []string, input string, interactive bool) (string, error) {
		calls = append(calls, call{name: name, args: args, input: input, interactive: interactive})
		if name == "fzf" {
			if !strings.HasPrefix(input, "0\tproject\trepo           \t"+project+"\n1\ttry    \t2026-09-29-test\t"+trial+"\n") {
				t.Fatalf("picker order: %q", input)
			}
			if !reflect.DeepEqual(args, []string{"--delimiter=\t", "--with-nth=2..", "--nth=2", "--tiebreak=index", "--tabstop=1"}) {
				t.Fatalf("unexpected fzf args: %q", args)
			}
			return "1\ttry    \t2026-09-29-test\t" + trial + "\n", nil
		}
		return "", nil
	}}
	if err := a.execute(nil); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || !reflect.DeepEqual(calls[1].args, []string{"new-session", "-A", "-s", "try-2026-09-29-test", "-c", trial}) {
		t.Fatalf("selected wrong session: %+v", calls)
	}
}

func TestTryCommandStartsSession(t *testing.T) {
	home := t.TempDir()
	var got []string
	a := app{home: home, insideTmux: false, today: func() time.Time {
		return time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	}, run: func(name string, args []string, input string, interactive bool) (string, error) {
		if name != "tmux" || !interactive {
			t.Fatalf("expected attached tmux, got %q %q", name, args)
		}
		got = args
		return "", nil
	}}
	if err := a.execute([]string{"try", "my idea"}); err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(home, "Projects", "tries", "2026-09-29-my-idea")
	want := []string{"new-session", "-A", "-s", "try-2026-09-29-my-idea", "-c", wantPath}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
