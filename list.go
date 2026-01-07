package main

import (
	"context"
	"io"

	"github.com/calvinalkan/agent-task/pkg/fs"
	flag "github.com/spf13/pflag"
)

// ListCmd returns the list command.
func ListCmd(cfg Config, fsys fs.FS, git *Git) *Command {
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.Bool("json", false, "Output as JSON")

	return &Command{
		Flags: flags,
		Usage: "list [flags]",
		Short: "List worktrees for current repo",
		Long:  `List all worktrees managed by wt for the current repository.`,
		Exec: func(_ context.Context, stdin io.Reader, stdout, stderr io.Writer, _ []string) error {
			return execList(stdin, stdout, stderr, cfg, fsys, git, flags)
		},
	}
}

func execList(_ io.Reader, stdout, _ io.Writer, _ Config, _ fs.FS, _ *Git, flags *flag.FlagSet) error {
	jsonOutput, _ := flags.GetBool("json")

	// Stub - implementation pending.
	_ = jsonOutput

	fprintln(stdout, "list: not implemented yet")

	return nil
}
