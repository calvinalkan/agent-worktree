package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	flag "github.com/spf13/pflag"
)

// Errors for init command.
var (
	errMissingShell     = errors.New("missing shell argument (usage: wt init bash)")
	errUnsupportedShell = errors.New("unsupported shell (supported: bash)")
	errTooManyInitArgs  = errors.New("too many arguments (usage: wt init bash)")
)

// InitCmd returns the init command.
func InitCmd() *Command {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")

	return &Command{
		Flags: flags,
		Usage: "init <shell>",
		Short: "Output shell integration code",
		Long: `Output shell integration code for the specified shell.

Add the output to your shell's config file:
  eval "$(wt init bash)"   # Add to ~/.bashrc

This enables:
  wt switch <name|id>      Change directory to a worktree
  wt create --switch       Create worktree and cd into it
  wt create -s             Short form of --switch

Supported shells: bash`,
		Exec: func(_ context.Context, _ io.Reader, stdout, _ io.Writer, args []string) error {
			return execInit(stdout, args)
		},
	}
}

func execInit(stdout io.Writer, args []string) error {
	if len(args) == 0 {
		return errMissingShell
	}

	if len(args) > 1 {
		return errTooManyInitArgs
	}

	shell := args[0]

	switch shell {
	case "bash":
		return outputBashInit(stdout)
	default:
		return fmt.Errorf("%w: %s", errUnsupportedShell, shell)
	}
}

// bashInitScript is the shell function that wraps wt for bash.
// It handles:
// - wt [global-flags] switch <name|id>: cd to worktree
// - wt [global-flags] create --switch/-s [...]: create and cd to worktree
// - All other commands: pass through to wt binary.
const bashInitScript = `wt() {
  local cmd="" cmd_pos=0 pos=0 skip_next=false has_switch=false

  # Find the command and check for --switch/-s flag
  for arg in "$@"; do
    if $skip_next; then
      skip_next=false
      ((pos++))
      continue
    fi
    case "$arg" in
      -C|--cwd|-c|--config)
        skip_next=true
        ;;
      -C=*|--cwd=*|-c=*|--config=*|-h|--help|-v|--version)
        ;;
      --switch|-s)
        has_switch=true
        ;;
      -*)
        # Unknown flag - could be command-specific, stop looking for cmd
        if [[ -z "$cmd" ]]; then
          cmd="$arg"
          cmd_pos=$pos
        fi
        ;;
      *)
        if [[ -z "$cmd" ]]; then
          cmd="$arg"
          cmd_pos=$pos
        fi
        ;;
    esac
    ((pos++))
  done

  if [[ "$cmd" == "switch" ]]; then
    local identifier="${@:$((cmd_pos + 2)):1}"
    if [[ -z "$identifier" ]]; then
      echo "error: missing worktree identifier (usage: wt switch <name|id>)" >&2
      return 1
    fi
    local global_flags=("${@:1:$cmd_pos}")
    local dir
    if dir="$(command wt "${global_flags[@]}" info "$identifier" --field path 2>&1)"; then
      cd "$dir" || return 1
    else
      echo "$dir" >&2
      return 1
    fi
  elif [[ "$cmd" == "create" && "$has_switch" == "true" ]]; then
    local dir
    if dir="$(command wt "$@" 2>&1)"; then
      cd "$dir" || return 1
    else
      echo "$dir" >&2
      return 1
    fi
  else
    command wt "$@"
  fi
}
`

func outputBashInit(stdout io.Writer) error {
	_, err := fmt.Fprint(stdout, bashInitScript)
	if err != nil {
		return fmt.Errorf("writing bash init script: %w", err)
	}

	return nil
}
