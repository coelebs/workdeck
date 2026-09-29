package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type call struct {
	name        string
	args        []string
	input       string
	interactive bool
}

func TestFindProjects(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "Projects", "project")
	worktree := filepath.Join(home, "Projects", "worktree")
	tooDeep := filepath.Join(home, "a", "b", "c", "d")
	ignored := filepath.Join(home, ".cache-work", "hidden")
	tries := filepath.Join(home, "Projects", "tries")
	for _, path := range []string{project, worktree, tooDeep, ignored, filepath.Join(tries, "with-git")} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{project, tooDeep, ignored, filepath.Join(tries, "with-git")} {
		if err := os.Mkdir(filepath.Join(path, ".git"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: elsewhere"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := findProjects(home, tries)
	if err != nil {
		t.Fatal(err)
	}
	want := []entry{
		{kind: "project", name: "worktree", path: worktree},
		{kind: "project", name: "project", path: project},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v; want %v", got, want)
	}
}

func TestPickerUsesIDNotDisplayPath(t *testing.T) {
	items := []entry{
		{kind: "project", name: "a", path: "/repo/a"},
		{kind: "project", name: "b", path: "/repo/a\nweird"},
	}
	a := app{run: func(name string, args []string, input string, interactive bool) (string, error) {
		if name != "fzf" || !strings.Contains(input, "1\tproject\tb\t/repo/a weird\n") {
			t.Fatalf("unexpected picker call %s %q %q", name, args, input)
		}
		return "1\tproject\tb\t/repo/a weird\n", nil
	}}
	got, ok, err := a.pick(items)
	if err != nil || !ok || got != items[1] {
		t.Fatalf("got %v, %v, %v", got, ok, err)
	}
}

func TestPickerCancel(t *testing.T) {
	a := app{run: func(string, []string, string, bool) (string, error) {
		return "", &execExitError{code: 130}
	}}
	_, ok, err := a.pick([]entry{{kind: "project", name: "a", path: "/repo/a"}})
	if ok || err != nil {
		t.Fatalf("cancel returned ok=%v err=%v", ok, err)
	}
}

func TestOpenSession(t *testing.T) {
	item := entry{kind: "project", path: "/home/vin/Projects/dotfiles"}
	for _, tc := range []struct {
		name   string
		inside bool
		exists bool
		want   [][]string
	}{
		{"outside", false, false, [][]string{{"new-session", "-A", "-s", "Projects-/dotfiles", "-c", item.path}}},
		{"inside existing", true, true, [][]string{{"has-session", "-t", "=Projects-/dotfiles"}, {"switch-client", "-t", "=Projects-/dotfiles"}}},
		{"inside new", true, false, [][]string{{"has-session", "-t", "=Projects-/dotfiles"}, {"new-session", "-d", "-s", "Projects-/dotfiles", "-c", item.path}, {"switch-client", "-t", "=Projects-/dotfiles"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []call
			a := app{insideTmux: tc.inside, run: func(name string, args []string, input string, interactive bool) (string, error) {
				calls = append(calls, call{name: name, args: args, interactive: interactive})
				if args[0] == "has-session" && !tc.exists {
					return "", &execExitError{code: 1}
				}
				return "", nil
			}}
			if err := a.openSession(item); err != nil {
				t.Fatal(err)
			}
			var got [][]string
			for _, call := range calls {
				if call.name != "tmux" || (call.args[0] == "new-session" && !tc.inside != call.interactive) {
					t.Fatalf("unexpected call %+v", call)
				}
				got = append(got, call.args)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDirectPath(t *testing.T) {
	path := t.TempDir()
	var got []string
	a := app{run: func(name string, args []string, input string, interactive bool) (string, error) {
		got = args
		return "", nil
	}}
	if err := a.execute([]string{path}); err != nil {
		t.Fatal(err)
	}
	if len(got) < 6 || got[0] != "new-session" || got[5] != path {
		t.Fatalf("wrong direct path: %v", got)
	}
	if err := a.execute([]string{path, path}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := a.execute([]string{filepath.Join(path, "missing")}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing path: %v", err)
	}
}
