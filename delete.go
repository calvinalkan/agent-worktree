package main

import (
	"context"
	"errors"
	"io"

	"github.com/calvinalkan/agent-task/pkg/fs"
	flag "github.com/spf13/pflag"
)

var errWorktreeNameRequired = errors.New("worktree name is required")

// DeleteCmd returns the delete command.
func DeleteCmd(cfg Config, fsys fs.FS) *Command {
	flags := flag.NewFlagSet("delete", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.Bool("force", false, "Delete even if worktree has uncommitted changes")
	flags.Bool("with-branch", false, "Also delete the git branch")

	return &Command{
		Flags: flags,
		Usage: "delete <name> [flags]",
		Short: "Delete a worktree",
		Long: `Delete a worktree by name.

If the worktree has uncommitted changes, use --force to proceed.
Use --with-branch to also delete the git branch.`,
		Exec: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args []string) error {
			return execDelete(ctx, stdin, stdout, stderr, cfg, fsys, flags, args)
		},
	}
}

func execDelete(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, cfg Config, fsys fs.FS, flags *flag.FlagSet, args []string) error {
	if len(args) == 0 {
		return errWorktreeNameRequired
	}

	name := args[0]
	force, _ := flags.GetBool("force")
	withBranch, _ := flags.GetBool("with-branch")

	// TODO: Implement
	_ = name
	_ = force
	_ = withBranch
	_ = fsys

	fprintln(stdout, "delete: not implemented yet")
	return nil
}
