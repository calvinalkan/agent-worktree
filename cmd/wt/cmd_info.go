package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/calvinalkan/agent-task/pkg/fs"
	flag "github.com/spf13/pflag"
)

// Errors for info command.
var (
	errNotInWorktree        = errors.New("this is a regular branch, not a worktree (use wt list to find worktrees)")
	errWorktreeInfoNotFound = errors.New("worktree info not found (.wt/worktree.json missing)")
	errInvalidField         = errors.New("invalid field (valid: name, agent_id, id, path, base_branch, created)")
)

// InfoCmd returns the info command.
func InfoCmd(cfg Config, fsys fs.FS, git *Git) *Command {
	flags := flag.NewFlagSet("info", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.Bool("json", false, "Output as JSON")
	flags.String("field", "", "Output single field: name, agent_id, id, path, base_branch, created")

	return &Command{
		Flags: flags,
		Usage: "info [flags]",
		Short: "Show current worktree info",
		Long: `Display information about the current worktree.

Must be run from within a wt-managed worktree (created by 'wt create').

Use --field for scripting, e.g.:
  wt info --field id      # Get worktree ID for port allocation
  wt info --field path    # Get absolute path to worktree`,
		Exec: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, _ []string) error {
			return execInfo(ctx, stdin, stdout, stderr, cfg, fsys, git, flags)
		},
	}
}

func execInfo(
	ctx context.Context,
	_ io.Reader,
	stdout, _ io.Writer,
	cfg Config,
	fsys fs.FS,
	git *Git,
	flags *flag.FlagSet,
) error {
	jsonOutput, _ := flags.GetBool("json")
	field, _ := flags.GetString("field")

	// Verify we're in a git repository
	_, err := git.RepoRoot(ctx, cfg.EffectiveCwd)
	if err != nil {
		return ErrNotGitRepository
	}

	// Find worktree root (look for .wt/worktree.json walking up)
	wtPath, err := findWorktreeRoot(fsys, cfg.EffectiveCwd)
	if err != nil {
		return errNotInWorktree
	}

	// Read worktree metadata
	info, err := readWorktreeInfo(fsys, wtPath)
	if err != nil {
		return fmt.Errorf("%w: %w", errWorktreeInfoNotFound, err)
	}

	// If --field is specified, output only that field
	if field != "" {
		return outputField(stdout, &info, wtPath, field)
	}

	// Full output
	if jsonOutput {
		return outputInfoJSON(stdout, &info, wtPath)
	}

	return outputInfoText(stdout, &info, wtPath)
}

// findWorktreeRoot walks up from startDir looking for .wt/worktree.json.
// Returns the worktree root directory path or an error if not found.
func findWorktreeRoot(fsys fs.FS, startDir string) (string, error) {
	dir := startDir

	for {
		infoPath := filepath.Join(dir, ".wt", "worktree.json")

		_, err := fsys.Stat(infoPath)
		if err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", errNotInWorktree
		}

		dir = parent
	}
}

func outputField(stdout io.Writer, info *WorktreeInfo, path, field string) error {
	switch field {
	case "name":
		fprintln(stdout, info.Name)
	case "agent_id":
		fprintln(stdout, info.AgentID)
	case "id":
		fprintln(stdout, info.ID)
	case "path":
		fprintln(stdout, path)
	case "base_branch":
		fprintln(stdout, info.BaseBranch)
	case "created":
		fprintln(stdout, info.Created.Format("2006-01-02T15:04:05Z"))
	default:
		return fmt.Errorf("%w: %s", errInvalidField, field)
	}

	return nil
}

func outputInfoText(stdout io.Writer, info *WorktreeInfo, path string) error {
	fprintf(stdout, "name:        %s\n", info.Name)
	fprintf(stdout, "agent_id:    %s\n", info.AgentID)
	fprintf(stdout, "id:          %d\n", info.ID)
	fprintf(stdout, "path:        %s\n", path)
	fprintf(stdout, "base_branch: %s\n", info.BaseBranch)
	fprintf(stdout, "created:     %s\n", info.Created.Format("2006-01-02T15:04:05Z"))

	return nil
}

type infoJSON struct {
	Name       string `json:"name"`
	AgentID    string `json:"agent_id"`
	ID         int    `json:"id"`
	Path       string `json:"path"`
	BaseBranch string `json:"base_branch"`
	Created    string `json:"created"`
}

func outputInfoJSON(stdout io.Writer, info *WorktreeInfo, path string) error {
	output := infoJSON{
		Name:       info.Name,
		AgentID:    info.AgentID,
		ID:         info.ID,
		Path:       path,
		BaseBranch: info.BaseBranch,
		Created:    info.Created.Format("2006-01-02T15:04:05Z"),
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")

	encodeErr := enc.Encode(output)
	if encodeErr != nil {
		return fmt.Errorf("encoding JSON: %w", encodeErr)
	}

	return nil
}
