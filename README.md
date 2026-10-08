# workdeck

Work sessions for Git projects and named, non-Git experiments ("tries"),
backed by tmux. The Go program uses fzf to pick an existing workspace.

## Usage

```sh
workdeck                         # Pick a Git project or existing try
workdeck ~/Projects/dotfiles     # Open a specific directory
workdeck try redis-pool           # Create or reopen today's named try
workdeck try 'test a library'     # Spaces become dashes
workdeck hotspare status --main ~/Work/deliverable-scb-tomahawk
workdeck --help
```

## Hot spares

Hot spares are independent, pre-warmed Git clones that Workdeck allocates for
editing large repositories. The main checkout remains available for
investigation; agents must claim a spare before editing. Unclaimed spares are
hidden from the normal `workdeck` picker. Claimed spares appear labelled as
`hotspare` entries.

Configure a managed set with an explicit release baseline:

```sh
workdeck hotspare setup --base origin/master \
  ~/Work/deliverable-scb-tomahawk \
  ~/Work/deliverable-scb-tomahawk-1 \
  ~/Work/deliverable-scb-tomahawk-2
```

Setup validates that every spare is a clean independent clone with the same
`origin` as main. It writes local state under `.git/`, does not alter Git
working state, and installs an AI skill at
`~/.agents/skills/workdeck-<main-name>/SKILL.md`. When main is not on
`master`, `--base` is required.

Claim a clean spare before editing. It fetches remotes, creates the requested
new branch from main's committed `HEAD`, and prints the allocated directory:

```sh
workdeck hotspare claim --main ~/Work/deliverable-scb-tomahawk AHWP-9999-fix
```

Uncommitted main changes are intentionally not copied. Claims fail if the
branch exists on origin or is checked out by a managed clone. Dirty or
misaligned spares are skipped. If none are available, Workdeck reports why;
do not edit main as a fallback.

```sh
workdeck hotspare status --main ~/Work/deliverable-scb-tomahawk
workdeck hotspare release --main ~/Work/deliverable-scb-tomahawk deliverable-scb-tomahawk-1
workdeck hotspare recover --main ~/Work/deliverable-scb-tomahawk deliverable-scb-tomahawk-1 --force
```

Release requires a clean, pushed branch merged into the configured base. It
fast-forwards the spare to that base without resetting, deleting branches, or
discarding files. Recovery only clears stale Workdeck state and locks; it never
changes Git state. Reinstall a generated skill with
`workdeck skill install --main <main>`; use `--force` only to replace a
manually edited skill.

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

## Install with Nix

With Nix and flakes enabled, run directly or install without cloning the repository:

```sh
nix run github:coelebs/workdeck -- --help
nix profile add github:coelebs/workdeck
```

The package includes fzf and tmux on its runtime PATH; you do not need to install
them separately. Both `packages.<system>.default` and
`packages.<system>.workdeck` export the package for `x86_64-linux` and
`aarch64-linux`.

To consume it from another flake, add an input and install its package (for
example, with Home Manager):

```nix
inputs.workdeck.url = "github:coelebs/workdeck";
inputs.workdeck.inputs.nixpkgs.follows = "nixpkgs";

# In a Home Manager module with access to the flake inputs:
home.packages = [ inputs.workdeck.packages.${pkgs.stdenv.hostPlatform.system}.default ];
```

The consumer's lock file pins the source revision. No local checkout or manually
compiled binary is needed. Update the pin with `nix flake update workdeck`.

To build or run from a local checkout:

```sh
nix build .#default
./result/bin/workdeck --help
nix run . -- --help
```
