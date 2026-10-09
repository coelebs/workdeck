package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %s: %v: %s", dir, strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func clone(t *testing.T, remote, path string) {
	t.Helper()
	cmd := exec.Command("git", "clone", remote, path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v: %s", err, output)
	}
}

func hotspareRepos(t *testing.T) (main string, spares []string) {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	cmd := exec.Command("git", "init", "--bare", remote)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v: %s", err, output)
	}
	seed := filepath.Join(root, "seed")
	runGit(t, root, "init", "-b", "master", seed)
	runGit(t, seed, "config", "user.name", "Test")
	runGit(t, seed, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(seed, "README"), []byte("base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "add", "README")
	runGit(t, seed, "commit", "-m", "base")
	runGit(t, seed, "remote", "add", "origin", remote)
	runGit(t, seed, "push", "-u", "origin", "master")

	main = filepath.Join(root, "main")
	clone(t, remote, main)
	for _, name := range []string{"spare-1", "spare-2"} {
		path := filepath.Join(root, name)
		clone(t, remote, path)
		spares = append(spares, path)
	}
	return main, spares
}

func TestHotspareSetupHidesInactiveAndInstallsSkill(t *testing.T) {
	main, spares := hotspareRepos(t)
	home := t.TempDir()
	a := app{home: home}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(main)
	if err != nil || len(config.Spares) != 2 || config.Base != "origin/master" {
		t.Fatalf("config = %+v, %v", config, err)
	}
	for _, spare := range spares {
		state, err := loadState(spare)
		if err != nil || state.Claimed {
			t.Fatalf("state %s = %+v, %v", spare, state, err)
		}
	}
	skill := filepath.Join(home, ".agents", "skills", "workdeck-main", "SKILL.md")
	content, err := os.ReadFile(skill)
	if err != nil || !strings.HasPrefix(string(content), "---\nname: workdeck-main\n") || !strings.Contains(string(content), "description:") || !strings.Contains(string(content), "hostspare claim --main "+main) {
		t.Fatalf("skill %s = %q, %v", skill, content, err)
	}
	openCodeSkill := filepath.Join(home, ".config", "opencode", "skills", "workdeck-main", "SKILL.md")
	openCodeContent, err := os.ReadFile(openCodeSkill)
	if err != nil || string(openCodeContent) != string(content) {
		t.Fatalf("OpenCode skill copy = %q, %v", openCodeContent, err)
	}
	items, err := activeHotspares([]entry{{kind: "project", name: "main", path: main}, {kind: "project", name: "spare-1", path: spares[0]}, {kind: "project", name: "spare-2", path: spares[1]}})
	if err != nil || len(items) != 1 || items[0].path != main {
		t.Fatalf("inactive picker entries = %+v, %v", items, err)
	}
}

func TestHotspareClaimAndRelease(t *testing.T) {
	main, spares := hotspareRepos(t)
	a := app{home: t.TempDir()}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	if err := claimHotspare(main, "task-branch"); err != nil {
		t.Fatal(err)
	}
	state, err := loadState(spares[0])
	if err != nil || !state.Claimed || state.Branch != "task-branch" {
		t.Fatalf("claim state = %+v, %v", state, err)
	}
	items, err := activeHotspares([]entry{{kind: "project", name: "main", path: main}, {kind: "project", name: "spare-1", path: spares[0]}, {kind: "project", name: "spare-2", path: spares[1]}})
	if err != nil || len(items) != 2 || items[1].kind != "hotspare" {
		t.Fatalf("claimed picker entries = %+v, %v", items, err)
	}

	if err := os.WriteFile(filepath.Join(spares[0], "task"), []byte("done\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, spares[0], "config", "user.name", "Test")
	runGit(t, spares[0], "config", "user.email", "test@example.com")
	runGit(t, spares[0], "add", "task")
	runGit(t, spares[0], "commit", "-m", "task")
	runGit(t, spares[0], "push", "origin", "task-branch")
	runGit(t, main, "fetch", "origin")
	runGit(t, main, "merge", "--ff-only", "origin/task-branch")
	runGit(t, main, "push", "origin", "master")

	if err := releaseHotspare(main, "spare-1"); err != nil {
		t.Fatal(err)
	}
	state, err = loadState(spares[0])
	if err != nil || state.Claimed {
		t.Fatalf("released state = %+v, %v", state, err)
	}
	if branch := runGit(t, spares[0], "branch", "--show-current"); branch != "master" {
		t.Fatalf("branch after release = %q", branch)
	}
}

func TestHotspareHooks(t *testing.T) {
	main, spares := hotspareRepos(t)
	a := app{home: t.TempDir()}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(t.TempDir(), "hook")
	content := "#!/bin/sh\nprintf '%s|%s|%s|%s|%s\\n' \"$1\" \"$WORKDECK_SPARE\" \"$WORKDECK_MAIN\" \"$WORKDECK_BRANCH\" \"$WORKDECK_BASE\" >> \"$2\"\n"
	if err := os.WriteFile(hook, []byte(content), 0700); err != nil {
		t.Fatal(err)
	}
	claimLog := filepath.Join(t.TempDir(), "claim.log")
	releaseLog := filepath.Join(t.TempDir(), "release.log")
	if err := configureHotspareHook([]string{"set", "--main", main, "--claim", hook, "claim", claimLog}); err != nil {
		t.Fatal(err)
	}
	if err := configureHotspareHook([]string{"set", "--main", main, "--release", hook, "release", releaseLog}); err != nil {
		t.Fatal(err)
	}
	if err := claimHotspare(main, "hook-branch"); err != nil {
		t.Fatal(err)
	}
	claim, err := os.ReadFile(claimLog)
	if err != nil || string(claim) != "claim|"+spares[0]+"|"+main+"|hook-branch|origin/master\n" {
		t.Fatalf("claim hook = %q, %v", claim, err)
	}
	runGit(t, spares[0], "config", "user.name", "Test")
	runGit(t, spares[0], "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(spares[0], "task"), []byte("done\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, spares[0], "add", "task")
	runGit(t, spares[0], "commit", "-m", "task")
	runGit(t, spares[0], "push", "origin", "hook-branch")
	runGit(t, main, "fetch", "origin")
	runGit(t, main, "merge", "--ff-only", "origin/hook-branch")
	runGit(t, main, "push", "origin", "master")
	if err := releaseHotspare(main, "spare-1"); err != nil {
		t.Fatal(err)
	}
	release, err := os.ReadFile(releaseLog)
	if err != nil || string(release) != "release|"+spares[0]+"|"+main+"|hook-branch|origin/master\n" {
		t.Fatalf("release hook = %q, %v", release, err)
	}
	if err := configureHotspareHook([]string{"clear", "--main", main, "--claim"}); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(main)
	if err != nil || len(config.OnClaim) != 0 || len(config.OnRelease) == 0 {
		t.Fatalf("config after clear = %+v, %v", config, err)
	}
}

func TestHotspareClaimRejectsRemoteBranch(t *testing.T) {
	main, spares := hotspareRepos(t)
	a := app{home: t.TempDir()}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	runGit(t, main, "checkout", "-b", "already-there")
	runGit(t, main, "push", "origin", "already-there")
	runGit(t, main, "checkout", "master")
	if err := claimHotspare(main, "already-there"); err == nil || !strings.Contains(err.Error(), "already exists on origin") {
		t.Fatalf("remote branch claim error = %v", err)
	}
}

func TestHotspareReleaseRefusesUnpushedChanges(t *testing.T) {
	main, spares := hotspareRepos(t)
	a := app{home: t.TempDir()}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	if err := claimHotspare(main, "unpublished"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(spares[0], "task"), []byte("pending\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, spares[0], "config", "user.name", "Test")
	runGit(t, spares[0], "config", "user.email", "test@example.com")
	runGit(t, spares[0], "add", "task")
	runGit(t, spares[0], "commit", "-m", "pending")
	if err := releaseHotspare(main, "spare-1"); err == nil || !strings.Contains(err.Error(), "has not been pushed") {
		t.Fatalf("release error = %v", err)
	}
}

func TestSkillInstallProtectsManualChanges(t *testing.T) {
	main, spares := hotspareRepos(t)
	home := t.TempDir()
	a := app{home: home}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".agents", "skills", "workdeck-main", "SKILL.md")
	if err := os.WriteFile(path, []byte("manual\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.installSkill(main, false); err == nil {
		t.Fatal("manual skill was overwritten")
	}
	if err := a.installSkill(main, true); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "If the current directory is listed as claimed, work there and do not claim another spare.") {
		t.Fatal("generated skill does not preserve active-spare guidance")
	}
}
