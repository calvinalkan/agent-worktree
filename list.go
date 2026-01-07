package main

import (
	"context"
	"io"

	"github.com/calvinalkan/agent-task/pkg/fs"
	flag "github.com/spf13/pflag"
)

// ListCmd returns the list command.
func ListCmd(cfg Config, fsys fs.FS) *Command {
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.Bool("json", false, "Output as JSON")

	return &Command{
		Flags: flags,
		Usage: "list [flags]",
		Short: "List worktrees for current repo",
		Long:  `List all worktrees managed by wt for the current repository.`,
		Exec: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args []string) error {
			return execList(ctx, stdin, stdout, stderr, cfg, fsys, flags)
		},
	}
}

func execList(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, cfg Config, fsys fs.FS, flags *flag.FlagSet) error {
	jsonOutput, _ := flags.GetBool("json")

	// TODO: Implement
	_ = jsonOutput
	_ = fsys

	fprintln(stdout, "list: not implemented yet")
	return nil
}
