# wt - Git Worktree Manager

A foundation for agentic coding workflows.

## Why wt?

Any multi-agent coding setup needs isolated workspaces. When you spin up parallel
agents—or even just concurrent dev tasks—each one needs:

- **Its own worktree** — complete file isolation, no conflicts, independent git state
- **A stable identity** — the `agent_id` (e.g. `swift-fox`) for logs, prompts, routing,
  inter-agent communication, reconciliation with external tools
- **A numeric seed** — the `id` for deterministic resource allocation: unique ports,
  database prefixes, container names, temp directories. Run full test suites in parallel
  without collisions—agent A's tests won't clobber agent B's database. ID assignment
  is atomic; no duplicates within a repo, ever.
- **Automated environment setup** — hooks handle dependency installation, Docker containers,
  `.env` configuration, database migrations, whatever your stack needs

`wt` provides this foundation. It's not an orchestrator or agent framework—it's the
solid, reliable piece underneath that handles worktree lifecycle and identity. Compose
it with your task runner, agent harness, or orchestration layer.

Built for humans and agents alike. Clean stdout/stderr separation: stdout is always
machine-parseable (paths, JSON), stderr carries context and errors. Agents can invoke
`wt` directly to self-manage their worktrees.

Built in Go. Thoroughly e2e tested.

---

## Specification

### Overview

`wt` is a CLI for managing git worktrees with auto-generated identifiers and lifecycle hooks.

---

### Worktree Context

All commands work from any directory within a repository or worktree, finding the nearest worktree scope automatically. For example, running `wt ls` from inside worktree "swift-fox" shows all worktrees for the repository, and `wt create` creates a new sibling worktree (not a nested one).

---

### Command Structure

```
wt [global-flags] <command> [command-flags] [arguments]
```

Global flags must appear before the command. Command flags must appear after the command.

---

### Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--cwd PATH` | `-C` | Run as if invoked from PATH |
| `--config PATH` | `-c` | Use config file at PATH instead of default |
| `--help` | `-h` | Show help (context-sensitive) |
| `--version` | `-v` | Show version and exit |

The `-h` / `--help` flag may appear anywhere in the command line. When present, help is displayed for the relevant command (or global help if no command specified) and no action is taken.

---

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success. Stdout is safe to parse. |
| 1 | Error. Stdout not safe to parse, check stderr for details. |
| 130 | Interrupted (Ctrl+C). Operation was cancelled. |

Stdout is always reserved for machine-readable output (paths, JSON, etc.). Stderr is always used for error messages and diagnostics.

---

### Signal Handling

When `wt` receives an interrupt signal (Ctrl+C / SIGINT / SIGTERM):

1. Current operation is cancelled gracefully
2. Message printed: "Interrupted, waiting up to 10s for cleanup..."
3. Waits up to 10 seconds for cleanup (e.g., hook termination, rollback)
4. A second Ctrl+C forces immediate exit
5. Exit code is 130

---

### Configuration

**Location** (in order of precedence, highest first):
1. Path specified by `--config` flag
2. Project config: `.wt/config.json` in repository root
3. User config: `$XDG_CONFIG_HOME/wt/config.json` (defaults to `~/.config/wt/config.json`)
4. Built-in defaults

Project and user configs are merged, with project config taking precedence for overlapping fields.

**Format**:
```json
{
  "base": "~/code/worktrees"
}
```

**Fields**:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `base` | string | `~/code/worktrees` | Base directory for worktrees |

**Behavior**:
- If config file does not exist, defaults are used
- If config file contains invalid JSON, exit with error

**Base path resolution**:
- Absolute path (starts with `/` or `~`): worktrees created at `<base>/<repo-name>/<worktree-name>/`
- Relative path: resolved relative to main repository root, worktrees created at `<base>/<worktree-name>/` (no repo name inserted)

---

### Directory Structure

**Per-repository** (committed to git):
```
.wt/
├── config.json         # project-level configuration (optional)
├── hooks/
│   ├── post-create     # executed after worktree creation
│   └── pre-delete      # executed before worktree deletion
```

**Per-worktree** (created by `wt create`):
```
.wt/
└── worktree.json       # worktree metadata (should be gitignored)
```

Hooks use shebang for interpreter selection and must be executable.

**Worktree metadata** (`.wt/worktree.json`):
```json
{
  "name": "my-feature",
  "agent_id": "swift-fox",
  "id": 42,
  "base_branch": "main",
  "created": "2025-01-07T16:30:00Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Worktree directory and branch name |
| `agent_id` | string | Auto-generated identifier (adjective-animal) |
| `id` | integer | Unique number for this worktree |
| `base_branch` | string | Branch the worktree was created from |
| `created` | string | ISO 8601 UTC timestamp |

---

### Naming

**agent_id**: Always auto-generated from word lists. Format: `<adjective>-<animal>` (e.g., `swift-fox`, `brave-owl`). Approximately 2,500 combinations available (50x50). Must be unique within the repository.

**name**: Defaults to `agent_id`. Can be overridden with `--name` flag.

**id**: Unique integer. Determined by scanning existing worktrees for the repository and using `max(id) + 1`. Starts at 1. No two worktrees for the same repository may have the same id; concurrent `wt create` operations must be handled safely.

**Collision handling**: If generated `agent_id` matches an existing `agent_id` or `name` in the repository's worktrees, regenerate with new random words. After 10 failed attempts, exit with error.

---

### Commands

#### `wt create`

Create a new worktree.

**Flags**:

| Flag | Short | Description |
|------|-------|-------------|
| `--name NAME` | `-n` | Custom worktree name (overrides agent_id for directory/branch) |
| `--from-branch BRANCH` | `-b` | Create from BRANCH (default: current branch) |
| `--with-changes` | | Copy uncommitted changes (staged, unstaged, and untracked files respecting .gitignore) to new worktree |

**Behavior**:

1. Verify current directory (or `-C` path) is within a git repository
2. Generate unique `id` (scan existing worktrees, use max + 1)
3. Generate `agent_id` from word lists
4. Set `name` to value of `--name` flag, or `agent_id` if not provided
5. Determine base branch (from `--from-branch` or current branch)
6. Create worktree base directory if it does not exist
7. Run `git worktree add -b <name> <path> <base-branch>`
8. Create `.wt/worktree.json` with metadata
9. If `--with-changes` specified, copy all uncommitted changes (staged, unstaged, and untracked files respecting .gitignore) to new worktree
10. If `.wt/hooks/post-create` exists and is executable, execute it
11. If hook exits non-zero, rollback: remove worktree and delete branch
12. Output worktree information

**Output** (success):
```
Created worktree:
  name:        swift-fox
  agent_id:    swift-fox
  id:          42
  path:        ~/code/worktrees/my-repo/swift-fox
  branch:      swift-fox
  from:        main
```

**Errors**:
- Not in a git repository: exit with error
- Git worktree add fails (e.g., branch already exists): exit with error
- Name collision after 10 retries: exit with error
- Cannot create base directory: exit with error
- Hook fails: rollback and exit with error

---

#### `wt ls`

List worktrees for the current repository.

**Flags**:

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

**Behavior**:

1. Verify current directory (or `-C` path) is within a git repository
2. Determine worktree base directory for this repository
3. Scan for existing worktrees
4. For each worktree, read `.wt/worktree.json` if present
5. Output worktree list

**Output** (default):
```
NAME            PATH                                              CREATED
swift-fox       ~/code/worktrees/my-repo/swift-fox                3 days ago
brave-owl       ~/code/worktrees/my-repo/brave-owl                1 hour ago
```

**Output** (`--json`):
```json
[
  {
    "name": "swift-fox",
    "agent_id": "swift-fox",
    "id": 42,
    "path": "/home/user/code/worktrees/my-repo/swift-fox",
    "base_branch": "main",
    "created": "2025-01-04T10:30:00Z"
  }
]
```

Only worktrees with `.wt/worktree.json` (created by `wt create`) are listed.

---

#### `wt info`

Display information about the current worktree.

**Flags**:

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--field FIELD` | Output only the specified field value |

**Behavior**:

1. Verify current directory (or `-C` path) is within a git worktree managed by wt
2. Read `.wt/worktree.json`
3. Output worktree information

**Output** (default):
```
name:        swift-fox
agent_id:    swift-fox
id:          42
path:        /home/user/code/worktrees/my-repo/swift-fox
base_branch: main
created:     2025-01-04T10:30:00Z
```

**Output** (`--field id`):
```
42
```

**Output** (`--json`):
```json
{
  "name": "swift-fox",
  "agent_id": "swift-fox",
  "id": 42,
  "path": "/home/user/code/worktrees/my-repo/swift-fox",
  "base_branch": "main",
  "created": "2025-01-04T10:30:00Z"
}
```

**Errors**:
- Not in a wt-managed worktree: exit with error
- `.wt/worktree.json` missing or invalid: exit with error

---

#### `wt delete <name>`

Delete a worktree.

**Arguments**:

| Argument | Description |
|----------|-------------|
| `name` | Name of the worktree to delete |

**Flags**:

| Flag | Description |
|------|-------------|
| `--force` | Delete even if worktree has uncommitted changes |
| `--with-branch` | Also delete the git branch |

**Behavior**:

1. Verify current directory (or `-C` path) is within a git repository
2. Locate worktree by name
3. If worktree has uncommitted changes and `--force` not provided: exit with error
4. If `.wt/hooks/pre-delete` exists and is executable, execute it
5. If hook exits non-zero: abort and exit with error
6. Run `git worktree remove <path>`
7. Output confirmation: "Deleted worktree directory: <path>"
8. Determine whether to delete branch:
   - If `--with-branch` provided: delete branch
   - If interactive terminal (tty): explain branch is safe, prompt user
   - If non-interactive: keep branch
9. If branch deleted, output: "Deleted branch: <name>"
10. Run `git worktree prune`

**Interactive prompt** (tty only):
```
Deleted worktree directory: /home/user/worktrees/my-repo/swift-fox

Branch 'swift-fox' still contains all your commits.
Also delete the branch? (y/N)
```

**Output** (success, without branch deletion):
```
Deleted worktree directory: /home/user/worktrees/my-repo/swift-fox
```

**Output** (success, with branch deletion):
```
Deleted worktree directory: /home/user/worktrees/my-repo/swift-fox
Deleted branch: swift-fox
```

**Errors**:
- Worktree not found: exit with error
- Uncommitted changes without `--force`: exit with error
- Hook fails: abort and exit with error

---

### Hooks

Hooks are executable files located in `.wt/hooks/`. They use shebang (`#!/bin/bash`, `#!/usr/bin/env python3`, etc.) to specify the interpreter.

**Available hooks**:

| Hook | When executed |
|------|---------------|
| `post-create` | After worktree creation, before success output |
| `pre-delete` | Before worktree removal |

**Environment variables** (available to all hooks):

| Variable | Description |
|----------|-------------|
| `WT_ID` | Unique worktree number |
| `WT_AGENT_ID` | Generated word combo identifier |
| `WT_NAME` | Worktree directory/branch name |
| `WT_PATH` | Absolute path to worktree (equals `$PWD`) |
| `WT_BASE_BRANCH` | Branch worktree was created from |
| `WT_REPO_ROOT` | Absolute path to main repository |

**Execution**:
- Hooks run with working directory set to the worktree (`$PWD` = `$WT_PATH`)
- All `WT_*` environment variables are available
- Hook stdout and stderr are displayed to the user (e.g., to show "Installing dependencies...")
- Hooks must be executable (`chmod +x`)
- If hook file does not exist, it is skipped (not an error)
- If hook file exists but is not executable, exit with error
- Hooks have a timeout of 5 minutes; if exceeded, the hook is killed and treated as failure
- Exit code 0 = success; any non-zero exit code = failure

**Cancellation**:
- When `wt` receives an interrupt signal (Ctrl+C), hooks receive SIGTERM
- Hooks can trap SIGTERM to perform cleanup (stop containers, remove temp files, etc.)
- Hooks have 7 seconds to exit after receiving SIGTERM
- If a hook does not exit within 7 seconds, it is forcibly killed with SIGKILL

**Failure handling**:
- `post-create` failure: worktree and branch are deleted (rollback), exit with error
- `pre-delete` failure: deletion is aborted, exit with error

---

### Error Conditions

| Condition | Behavior |
|-----------|----------|
| Not in a git repository | Exit with error |
| Config file invalid JSON | Exit with error |
| Config file missing | Use defaults |
| Base directory cannot be created | Exit with error |
| Name collision (10 retries) | Exit with error |
| Git operation fails | Exit with error |
| Hook exists but not executable | Exit with error |
| Hook fails (non-zero exit) | Rollback/abort, exit with error |
| Delete dirty worktree without `--force` | Exit with error |
| Worktree not found (delete) | Exit with error |

---

### Examples

**Create worktree with defaults**:
```bash
$ wt create
Created worktree:
  name:        swift-fox
  agent_id:    swift-fox
  id:          1
  path:        ~/code/worktrees/my-repo/swift-fox
  branch:      swift-fox
  from:        main
```

**Create worktree with custom name from specific branch**:
```bash
$ wt create -n feature-auth -b develop
Created worktree:
  name:        feature-auth
  agent_id:    brave-owl
  id:          2
  path:        ~/code/worktrees/my-repo/feature-auth
  branch:      feature-auth
  from:        develop
```

**Create worktree and copy uncommitted changes**:
```bash
$ wt create --with-changes
```

**List worktrees**:
```bash
$ wt ls
NAME            PATH                                              CREATED
swift-fox       ~/code/worktrees/my-repo/swift-fox                3 days ago
feature-auth    ~/code/worktrees/my-repo/feature-auth             1 hour ago
```

**List worktrees as JSON**:
```bash
$ wt ls --json
```

**Show current worktree info**:
```bash
$ wt info
name:        swift-fox
agent_id:    swift-fox
id:          42
path:        /home/user/code/worktrees/my-repo/swift-fox
base_branch: main
created:     2025-01-04T10:30:00Z
```

**Get specific field**:
```bash
$ wt info --field id
42
```

**Delete worktree**:
```bash
$ wt delete swift-fox
Deleted worktree directory: /home/user/worktrees/my-repo/swift-fox

Branch 'swift-fox' still contains all your commits.
Also delete the branch? (y/N) y
Deleted branch: swift-fox
```

**Delete worktree non-interactively**:
```bash
$ wt delete swift-fox --with-branch
Deleted worktree directory: /home/user/worktrees/my-repo/swift-fox
Deleted branch: swift-fox
```

**Delete worktree with uncommitted changes**:
```bash
$ wt delete feature-auth
Error: worktree has uncommitted changes (use --force to override)

$ wt delete feature-auth --force --with-branch
Deleted worktree directory: /home/user/worktrees/my-repo/feature-auth
Deleted branch: feature-auth
```

**Use custom config**:
```bash
$ wt -c ./project-config.json create
```

**Run from different directory**:
```bash
$ wt -C ~/code/other-repo ls
```

---

### Example Hooks

**`.wt/hooks/post-create`** (Node.js project):
```bash
#!/bin/bash
set -e
npm install
cp "$WT_REPO_ROOT/.env" .env 2>/dev/null || true
```

**`.wt/hooks/post-create`** (with Docker):
```bash
#!/bin/bash
set -e
bun install

# Use WT_ID for unique port
PORT_OFFSET=$((WT_ID * 10))
sed -i "s/5432/$((5432 + PORT_OFFSET))/g" .env

docker compose -p "db-$WT_ID" up -d
bun run migrate
```

**`.wt/hooks/pre-delete`**:
```bash
#!/bin/bash
docker compose -p "db-$WT_ID" down -v 2>/dev/null || true
```
