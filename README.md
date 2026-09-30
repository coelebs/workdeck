# workdeck

Work sessions for Git projects and named, non-Git experiments ("tries"),
backed by tmux. The Go program uses fzf to pick an existing workspace.

## Usage

```sh
workdeck                         # Pick a Git project or existing try
workdeck ~/Projects/dotfiles     # Open a specific directory
workdeck try redis-pool           # Create or reopen today's named try
workdeck try 'test a library'     # Spaces become dashes
workdeck --help
```

`try` always requires a name. A new try named `redis-pool` is stored as
`YYYY-MM-DD-redis-pool` and is **not** initialized as a Git repository.
Invoking it again on the same day reopens the same directory and tmux session
without deleting its contents. A try on a later day gets its own directory.
Choose a tries location outside any parent Git repository if you want the
directories to be wholly independent of Git.

Set `WORKDECK_TRIES_DIR` to change the location (default
`~/Projects/tries/`). Both absolute paths and `~/...` paths work:

```sh
export WORKDECK_TRIES_DIR="$HOME/Projects/experiments"
```

The picker searches Git projects under `$HOME` to depth four (including Git
worktrees with `.git` files), skipping dot-directories and `ncs*` paths.
It also lists immediate child directories of the tries location, even though
they do not contain `.git`. Projects appear first; tries are labelled `try`.
fzf searches their names, not the tries root, and uses input order to break
equal-scoring matches in favor of projects. A much better try match can still
outrank a project; fzf does not support a numeric penalty per entry.

Outside tmux the program creates or attaches to a session. Inside tmux it
creates a detached session if needed, then switches the client. Existing
project session names retain the old script's naming format; tries get distinct
`try-YYYY-MM-DD-name` session names.

## Development

The Go sources and tests live in `src/`. Run `direnv allow` once to let
`.envrc` load the pinned Nix dev shell automatically, or use `nix develop`
manually. The shell provides Go, fzf, tmux and Git. From the repository root:

```sh
go test ./...
go vet ./...
go build -o workdeck ./src
./workdeck --help
```

Without direnv:

```sh
nix develop --command go test ./...
nix develop --command go build -o workdeck ./src
```

Tests create temporary projects and tries. The CLI test runs real fzf in
noninteractive filter mode with a fake tmux; it does not create live tmux
sessions.

## Nix package and dotfiles

The flake builds the executable from `src/` without a manually compiled binary:

```sh
nix build .#default
./result/bin/workdeck --help
```

The companion dotfiles flake points to this local checkout and installs this
package through Home Manager. Its Ctrl-F bindings invoke `workdeck` instead of
the old `tmux-sessionizer` script. Until you activate those changes, your
existing shortcuts are unaffected. If you move this checkout, update the
dotfiles flake input path; refresh its lock after workdeck source changes.
