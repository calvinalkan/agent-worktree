package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	flag "github.com/spf13/pflag"
)

// Command defines a CLI command with unified help generation.
type Command struct {
	// Flags defines command-specific flags.
	Flags *flag.FlagSet

	// Usage is the freeform usage string shown after "wt" in help.
	// Includes the command name and arguments/flags.
	// Examples: "create [flags]", "list [flags]", "delete <name> [flags]"
	Usage string

	// Short is a one-line description for the global help listing.
	Short string

	// Long is the full description shown in command help.
	// If empty, Short is used instead.
	Long string

	// Exec runs the command after flags are parsed.
	Exec func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args []string) error
}

// Name returns the command name (first word of Usage).
func (c *Command) Name() string {
	name, _, _ := strings.Cut(c.Usage, " ")
	return name
}

// HelpLine returns the short help line for the main usage display.
func (c *Command) HelpLine() string {
	return fmt.Sprintf("  %-22s %s", c.Usage, c.Short)
}

// PrintHelp prints the full help output for "wt <cmd> --help".
func (c *Command) PrintHelp(w io.Writer) {
	fprintln(w, "Usage: wt", c.Usage)
	fprintln(w)

	desc := c.Long
	if desc == "" {
		desc = c.Short
	}
	fprintln(w, desc)

	if c.Flags != nil && c.Flags.HasFlags() {
		fprintln(w)
		fprintln(w, "Flags:")

		var buf strings.Builder
		c.Flags.SetOutput(&buf)
		c.Flags.PrintDefaults()
		fprintf(w, "%s", buf.String())
	}
}

// Run parses flags and executes the command. Returns exit code.
func (c *Command) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	c.Flags.SetOutput(&strings.Builder{}) // discard pflag output

	err := c.Flags.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			c.PrintHelp(stdout)
			return 0
		}

		fprintln(stderr, "error:", err)
		fprintln(stderr)
		c.PrintHelp(stderr)
		return 1
	}

	// Check if help was requested
	if help, _ := c.Flags.GetBool("help"); help {
		c.PrintHelp(stdout)
		return 0
	}

	if err := c.Exec(ctx, stdin, stdout, stderr, c.Flags.Args()); err != nil {
		fprintln(stderr, "error:", err)
		return 1
	}

	return 0
}
