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
	// Git does not copy repository-local identity settings when cloning.
	// Every test clone must be able to commit without a global user config.
	runGit(t, path, "config", "user.name", "Test")
	runGit(t, path, "config", "user.email", "test@example.com")
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

	if err := releaseHotspare(main, "spare-1", false); err != nil {
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

func TestHotspareReleaseAcceptsSquashMergedBranch(t *testing.T) {
	main, spares := hotspareRepos(t)
	a := app{home: t.TempDir()}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	if err := claimHotspare(main, "task-branch"); err != nil {
		t.Fatal(err)
	}
	runGit(t, spares[0], "config", "user.name", "Test")
	runGit(t, spares[0], "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(spares[0], "task"), []byte("done\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, spares[0], "add", "task")
	runGit(t, spares[0], "commit", "-m", "task")
	runGit(t, spares[0], "push", "origin", "task-branch")
	runGit(t, main, "fetch", "origin")
	runGit(t, main, "merge", "--squash", "origin/task-branch")
	runGit(t, main, "config", "user.name", "Test")
	runGit(t, main, "config", "user.email", "test@example.com")
	runGit(t, main, "commit", "-m", "squashed task")
	runGit(t, main, "push", "origin", "master")

	if err := releaseHotspare(main, "spare-1", false); err != nil {
		t.Fatal(err)
	}
	state, err := loadState(spares[0])
	if err != nil || state.Claimed {
		t.Fatalf("released state = %+v, %v", state, err)
	}
}

func TestHotsparePauseArchivesUnmergedCommittedWork(t *testing.T) {
	main, spares := hotspareRepos(t)
	a := app{home: t.TempDir()}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	if err := claimHotspare(main, "later"); err != nil {
		t.Fatal(err)
	}
	runGit(t, spares[0], "config", "user.name", "Test")
	runGit(t, spares[0], "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(spares[0], "later"), []byte("keep this\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, spares[0], "add", "later")
	runGit(t, spares[0], "commit", "-m", "finish later")

	if err := pauseHotspare(main, "spare-1"); err != nil {
		t.Fatal(err)
	}
	state, err := loadState(spares[0])
	if err != nil || state.Claimed {
		t.Fatalf("paused state = %+v, %v", state, err)
	}
	if branch := runGit(t, spares[0], "branch", "--show-current"); branch != "master" {
		t.Fatalf("branch after pause = %q", branch)
	}
	if gitOK(spares[0], "show-ref", "--verify", "--quiet", "refs/heads/later") {
		t.Fatal("paused branch still exists locally")
	}
	paused, err := pausedHotspares(main)
	if err != nil || len(paused) != 1 || paused[0].Branch != "later" {
		t.Fatalf("paused archives = %+v, %v", paused, err)
	}
	if _, err := os.Stat(paused[0].Patch); err != nil {
		t.Fatalf("patch missing: %v", err)
	}
	runGit(t, spares[0], "am", paused[0].Patch)
	content, err := os.ReadFile(filepath.Join(spares[0], "later"))
	if err != nil || string(content) != "keep this\n" {
		t.Fatalf("applied patch = %q, %v", content, err)
	}
}

func TestHotspareResumeRestoresPausedBranch(t *testing.T) {
	main, spares := hotspareRepos(t)
	a := app{home: t.TempDir()}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	if err := claimHotspare(main, "later"); err != nil {
		t.Fatal(err)
	}
	runGit(t, spares[0], "config", "user.name", "Test")
	runGit(t, spares[0], "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(spares[0], "later"), []byte("restore me\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, spares[0], "add", "later")
	runGit(t, spares[0], "commit", "-m", "later")
	if err := pauseHotspare(main, "spare-1"); err != nil {
		t.Fatal(err)
	}
	if err := resumeHotspare(main, "later"); err != nil {
		t.Fatal(err)
	}
	state, err := loadState(spares[0])
	if err != nil || !state.Claimed || state.Branch != "later" {
		t.Fatalf("resumed state = %+v, %v", state, err)
	}
	content, err := os.ReadFile(filepath.Join(spares[0], "later"))
	if err != nil || string(content) != "restore me\n" {
		t.Fatalf("restored content = %q, %v", content, err)
	}
}

func TestHotsparePauseRefusesUncommittedChanges(t *testing.T) {
	main, spares := hotspareRepos(t)
	a := app{home: t.TempDir()}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	if err := claimHotspare(main, "later"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(spares[0], "README"), []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := pauseHotspare(main, "spare-1"); err == nil || !strings.Contains(err.Error(), "tracked uncommitted changes") {
		t.Fatalf("pause error = %v", err)
	}
	state, err := loadState(spares[0])
	if err != nil || !state.Claimed {
		t.Fatalf("state after refused pause = %+v, %v", state, err)
	}
}

func TestHotsparePauseArchivesDevtoolPatchAndDiscardsUntrackedFiles(t *testing.T) {
	main, spares := hotspareRepos(t)
	a := app{home: t.TempDir()}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	if err := claimHotspare(main, "later"); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(spares[0], "work", "build", "workspace", "sources", "recipe")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, source, "init", "-b", "master")
	runGit(t, source, "config", "user.name", "Test")
	runGit(t, source, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(source, "source"), []byte("base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, source, "add", "source")
	runGit(t, source, "commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(source, "source"), []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(spares[0], "discard-me"), []byte("temporary\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runner := filepath.Join(spares[0], "ares-build-agent", "run-build-agent.py")
	if err := os.MkdirAll(filepath.Dir(runner), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runner, []byte("#!/bin/sh\nrm -rf work/build/workspace\n"), 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, spares[0], "add", "ares-build-agent/run-build-agent.py")
	runGit(t, spares[0], "commit", "-m", "test build agent")

	if err := pauseHotspare(main, "spare-1"); err != nil {
		t.Fatal(err)
	}
	paused, err := pausedHotspares(main)
	if err != nil || len(paused) != 1 || len(paused[0].DevtoolPatches) != 1 {
		t.Fatalf("paused archives = %+v, %v", paused, err)
	}
	if _, err := os.Stat(paused[0].DevtoolPatches[0].Patch); err != nil {
		t.Fatalf("devtool patch missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(spares[0], "work", "build", "workspace")); !os.IsNotExist(err) {
		t.Fatalf("workspace remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(spares[0], "discard-me")); !os.IsNotExist(err) {
		t.Fatalf("untracked file remains: %v", err)
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
	if err := releaseHotspare(main, "spare-1", false); err != nil {
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

func TestHotspareReleaseAcceptsUnpublishedRenamedBranch(t *testing.T) {
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
	runGit(t, spares[0], "branch", "-m", "renamed")
	commit := runGit(t, spares[0], "rev-parse", "HEAD")
	if err := releaseHotspare(main, "spare-1", false); err != nil {
		t.Fatal(err)
	}
	if got := runGit(t, spares[0], "rev-parse", "renamed"); got != commit {
		t.Fatalf("unpublished branch changed: %s", got)
	}
	state, err := loadState(spares[0])
	if err != nil || state.Claimed {
		t.Fatalf("released state = %+v, %v", state, err)
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
	for _, text := range []string{"trust that decision", "Ask for explicit confirmation", "<spare> --force", "Never push"} {
		if !strings.Contains(string(content), text) {
			t.Fatalf("generated skill missing %q", text)
		}
	}
	if strings.Contains(string(content), "Release only after") {
		t.Fatal("generated skill still requires push/merge verification")
	}
}

func TestHotspareReleaseDirtyChangesRequireForce(t *testing.T) {
	for _, kind := range []string{"staged", "unstaged", "untracked"} {
		t.Run(kind, func(t *testing.T) {
			main, spares := hotspareRepos(t)
			a := app{home: t.TempDir()}
			if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
				t.Fatal(err)
			}
			if err := claimHotspare(main, "done"); err != nil {
				t.Fatal(err)
			}
			name := "README"
			if kind == "untracked" {
				name = "untracked"
			}
			if err := os.WriteFile(filepath.Join(spares[0], name), []byte("changed\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if kind == "staged" {
				runGit(t, spares[0], "add", name)
			}
			runGit(t, spares[0], "config", "core.excludesFile", filepath.Join(t.TempDir(), "excludes"))
			// Ignored build output must survive force cleanup.
			exclude := filepath.Join(spares[0], ".git", "info", "exclude")
			if err := os.WriteFile(exclude, []byte("build-output\n"), 0644); err != nil {
				t.Fatal(err)
			}
			build := filepath.Join(spares[0], "build-output")
			if err := os.WriteFile(build, []byte("keep\n"), 0644); err != nil {
				t.Fatal(err)
			}
			before := runGit(t, spares[0], "status", "--porcelain")
			if err := a.hotspare([]string{"release", "--main", main, "spare-1"}); err == nil || !strings.Contains(err.Error(), "--force") || !strings.Contains(err.Error(), name) {
				t.Fatalf("release error = %v", err)
			}
			if after := runGit(t, spares[0], "status", "--porcelain"); after != before {
				t.Fatal("refused release changed files")
			}
			state, err := loadState(spares[0])
			if err != nil || !state.Claimed {
				t.Fatalf("state after refusal = %+v, %v", state, err)
			}
			if err := a.hotspare([]string{"release", "--main", main, "--force", "spare-1"}); err != nil {
				t.Fatal(err)
			}
			if status := runGit(t, spares[0], "status", "--porcelain"); status != "" {
				t.Fatalf("dirty after force: %s", status)
			}
			if _, err := os.Stat(build); err != nil {
				t.Fatalf("ignored output removed: %v", err)
			}
		})
	}
}

func TestHotspareForceReleaseCleansSubmodule(t *testing.T) {
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	main, spares := hotspareRepos(t)
	// Use an existing local repository as a submodule source.
	runGit(t, main, "config", "user.name", "Test")
	runGit(t, main, "config", "user.email", "test@example.com")
	runGit(t, main, "submodule", "add", spares[1], "module")
	runGit(t, main, "commit", "-am", "add submodule")
	runGit(t, main, "push", "origin", "master")
	for _, spare := range spares {
		runGit(t, spare, "fetch", "origin")
		runGit(t, spare, "merge", "--ff-only", "origin/master")
		runGit(t, spare, "submodule", "update", "--init")
	}
	a := app{home: t.TempDir()}
	if err := a.setupHotspares(append([]string{"--base", "origin/master", main}, spares...)); err != nil {
		t.Fatal(err)
	}
	if err := claimHotspare(main, "done"); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(spares[0], "module")
	runGit(t, module, "config", "user.name", "Test")
	runGit(t, module, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(module, "README"), []byte("new commit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, module, "commit", "-am", "drift")
	if err := os.WriteFile(filepath.Join(module, "README"), []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "untracked"), []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// A local ignore setting must not hide submodule changes from release.
	runGit(t, spares[0], "config", "submodule.module.ignore", "all")
	if err := releaseHotspare(main, "spare-1", false); err == nil || !strings.Contains(err.Error(), "module") {
		t.Fatalf("release error = %v", err)
	}
	if err := releaseHotspare(main, "spare-1", true); err != nil {
		t.Fatal(err)
	}
	if status := runGit(t, spares[0], "status", "--porcelain", "--ignore-submodules=none"); status != "" {
		t.Fatalf("dirty after force: %s", status)
	}
}
