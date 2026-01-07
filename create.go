package main

import (
	"context"
	"io"

	"github.com/calvinalkan/agent-task/pkg/fs"
	flag "github.com/spf13/pflag"
)

// CreateCmd returns the create command.
func CreateCmd(cfg Config, fsys fs.FS) *Command {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.StringP("name", "n", "", "Custom worktree name")
	flags.StringP("from-branch", "b", "", "Create from branch (default: current branch)")
	flags.Bool("copy-changes", false, "Copy uncommitted changes to new worktree")

	return &Command{
		Flags: flags,
		Usage: "create [flags]",
		Short: "Create a new worktree",
		Long: `Create a new worktree with auto-generated name and unique ID.

A random agent_id is generated (e.g., swift-fox) and used as the default
worktree name. Use --name to override.`,
		Exec: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args []string) error {
			return execCreate(ctx, stdin, stdout, stderr, cfg, fsys, flags)
		},
	}
}

func execCreate(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, cfg Config, fsys fs.FS, flags *flag.FlagSet) error {
	name, _ := flags.GetString("name")
	fromBranch, _ := flags.GetString("from-branch")
	copyChanges, _ := flags.GetBool("copy-changes")

	// TODO: Implement
	_ = name
	_ = fromBranch
	_ = copyChanges
	_ = fsys

	fprintln(stdout, "create: not implemented yet")
	return nil
}
