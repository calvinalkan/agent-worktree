---
schema_version: 1
id: d5fs9hr
status: closed
closed: 2026-01-08T12:03:21Z
blocked-by: [d5fs9f0, d5fs080]
created: 2026-01-08T11:28:07Z
type: feature
priority: 2
---
# Add wt init <shell> for shell integration

Add `wt init bash` (and zsh/fish) command that outputs shell integration code.

Usage:
  eval "$(wt init bash)"   # in .bashrc
  eval "$(wt init zsh)"    # in .zshrc

The output defines a shell function that wraps `wt` to enable:
  wt switch <name|id>     # cd to worktree
  wt create -s/--switch   # create and cd into new worktree

Example output for bash:
```bash
wt() {
  case "$1" in
    switch)
      local dir
      dir="$(command wt info "$2" --field path)" && cd "$dir"
      ;;
    create)
      if [[ " $* " =~ " --switch " || " $* " =~ " -s " ]]; then
        local out
        out="$(command wt create "${@:2}")" && echo "$out" && cd "$(echo "$out" | jq -r .path)"
      else
        command wt "$@"
      fi
      ;;
    *)
      command wt "$@"
      ;;
  esac
}
```

## Acceptance Criteria

- Support bash, zsh, fish
- `wt switch <name|id>` changes directory to worktree
- `wt create --switch/-s` creates and switches to new worktree
- Error handling for not found / create failures
- Document in README how to add to shell config
