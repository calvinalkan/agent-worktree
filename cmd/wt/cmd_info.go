package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/calvinalkan/agent-task/pkg/fs"
	flag "github.com/spf13/pflag"
)

// Errors for info command.
var (
	errNotInWorktree        = errors.New("this is a regular branch, not a worktree (use wt list to find worktrees)")
	errInvalidField         = errors.New("invalid field (valid: name, agent_id, id, path, base_branch, created)")
	errWorktreeNotFoundInfo = errors.New("worktree not found")
)

// InfoCmd returns the info command.
func InfoCmd(cfg Config, fsys fs.FS, git *Git) *Command {
	flags := flag.NewFlagSet("info", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.Bool("json", false, "Output as JSON")
	flags.String("field", "", "Output single field: name, agent_id, id, path, base_branch, created")

	return &Command{
		Flags: flags,
		Usage: "info [identifier] [flags]",
		Short: "Show worktree info",
		Long: `Display information about a worktree.

Without arguments, shows info for the current worktree (must be inside a
wt-managed worktree created by 'wt create').

With an identifier argument, looks up any worktree by:
  • name      - the worktree directory/branch name
  • agent_id  - the generated identifier (e.g., swift-fox)  
  • id        - the numeric ID (e.g., 3)

Examples:
  wt info                     # Current worktree
  wt info swift-fox           # Lookup by name or agent_id
  wt info 3                   # Lookup by numeric ID
  wt info --field id          # Get worktree ID for port allocation
  wt info foo --field path    # Get path for a specific worktree`,
		Exec: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args []string) error {
			return execInfo(ctx, stdin, stdout, stderr, cfg, fsys, git, flags, args)
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
	args []string,
) error {
	jsonOutput, _ := flags.GetBool("json")
	field, _ := flags.GetString("field")

	// Get main repo root (works from inside worktrees too)
	mainRepoRoot, err := git.MainRepoRoot(ctx, cfg.EffectiveCwd)
	if err != nil {
		return err
	}

	var info WorktreeInfo

	var wtPath string

	if len(args) > 0 {
		// Lookup by identifier
		identifier := args[0]

		baseDir := resolveWorktreeBaseDir(cfg, mainRepoRoot)

		worktrees, findErr := findWorktreesWithPaths(fsys, baseDir)
		if findErr != nil {
			return fmt.Errorf("scanning worktrees: %w", findErr)
		}

		wt, found := findWorktreeByIdentifier(worktrees, identifier)
		if !found {
			return fmt.Errorf("%w: %s", errWorktreeNotFoundInfo, identifier)
		}

		info = wt.WorktreeInfo
		wtPath = wt.Path
	} else {
		// Current worktree mode
		wtPath, err = findWorktreeRoot(fsys, cfg.EffectiveCwd)
		if err != nil {
			return errNotInWorktree
		}

		info, err = readWorktreeInfo(fsys, wtPath)
		if err != nil {
			return err
		}
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

// findWorktreeByIdentifier searches worktrees by name, agent_id, or numeric id.
func findWorktreeByIdentifier(worktrees []WorktreeWithPath, identifier string) (WorktreeWithPath, bool) {
	// Try numeric ID first
	id, err := strconv.Atoi(identifier)
	if err == nil {
		for _, wt := range worktrees {
			if wt.ID == id {
				return wt, true
			}
		}
	}

	// Try name or agent_id
	for _, wt := range worktrees {
		if wt.Name == identifier || wt.AgentID == identifier {
			return wt, true
		}
	}

	return WorktreeWithPath{}, false
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
