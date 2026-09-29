# tmux-sessionizer

A project and scratch-directory picker for tmux. It keeps the original
`tmux-sessionizer` command shape and uses `fzf` for selection. Requires Go to
build; at runtime it needs `fzf` for the picker and `tmux` to open sessions.

## Usage

```sh
tmux-sessionizer                       # Pick a Git project or an existing try
tmux-sessionizer ~/Projects/dotfiles   # Open a specific directory
tmux-sessionizer try redis-pool         # Create or reopen today's named try
tmux-sessionizer try 'test a library'   # Spaces become dashes
tmux-sessionizer --help
```

`try` always requires a name. A new try named `redis-pool` is stored as
`YYYY-MM-DD-redis-pool` and is **not** initialized as a Git repository.
Invoking it again on the same day reopens the same directory and tmux session
without deleting its contents. A try on a later day gets its own directory.
Choose a tries location outside any parent Git repository if you want the
directories to be wholly independent of Git.

Set `TMUX_SESSIONIZER_TRIES_DIR` to change the location (default
`~/src/tries`). Both absolute paths and `~/...` paths work:

```sh
export TMUX_SESSIONIZER_TRIES_DIR="$HOME/Projects/experiments"
```

The picker searches Git projects under `$HOME` to depth four (including Git
worktrees with `.git` files), skipping `.trash*`, `ncs*` and `.cache*` paths.
It also lists immediate child directories of the tries location, even though
they do not contain `.git`. Projects appear first; tries are labelled `try`.
fzf searches their names, not the tries root, and uses input order to break
equal-scoring matches in favor of projects. A much better try match can still
outrank a project; fzf does not support a numeric penalty per entry.

Outside tmux the program creates or attaches to a session. Inside tmux it
creates a detached session if needed, then switches the client. Existing
project session names retain the old script's naming format; tries get distinct
`try-YYYY-MM-DD-name` session names.

## Build and verify

Enter the repository with direnv enabled and allowed to load `.envrc`; Go,
fzf, tmux and Git will come from the pinned Nix dev shell. You can also enter
it manually with `nix develop` without direnv. After that:

```sh
go test ./...
go vet ./...
go build -o tmux-sessionizer .
./tmux-sessionizer --help
./tmux-sessionizer try my-test
./tmux-sessionizer          # Check that my-test appears in the picker
```

If Go is not installed but Nix is available:

```sh
nix develop --command go test ./...
nix develop --command go build -o tmux-sessionizer .
```

`direnv` and the `nix-direnv` integration are enabled in the companion
dotfiles Home Manager shell configuration. After applying that configuration,
run `direnv allow` **once** in this directory to trust `.envrc`. Thereafter
the shell loads on entry and unloads on exit. The allow step is intentionally
not automatic. If you have not applied Home Manager yet, `nix develop` works
on its own.

Tests create temporary projects and tries. The CLI test runs real fzf in
noninteractive filter mode with a fake tmux; it does not create live tmux
sessions. To test a particular tries location without changing your shell
configuration, prefix the binary invocation with
`TMUX_SESSIONIZER_TRIES_DIR=/your/path`.

## Activating the existing shortcuts

The dotfiles repository supplies `~/.local/bin/tmux-sessionizer` through Home
Manager, so building here **does not replace** that command. Do not write a
binary into the Home Manager-managed `~/.local/bin` symlink. After verifying
this build, replace the old script in
`~/Projects/dotfiles/bin/.local/bin/tmux-sessionizer` with a launcher such as:

```sh
#!/bin/sh
exec "$HOME/Projects/tmux-sessionizer/tmux-sessionizer" "$@"
```

Then apply your Home Manager configuration. The existing Ctrl-F bindings can
keep invoking `tmux-sessionizer`. Keep the built binary at that path (or adjust
the launcher when you install the program elsewhere). This activation is not
performed automatically, so your current shortcuts keep working while you
review this repository.
