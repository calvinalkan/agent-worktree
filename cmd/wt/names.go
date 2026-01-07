package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
)

// ErrNameGenerationFailed is returned when a unique agent_id cannot be generated.
var ErrNameGenerationFailed = errors.New("generating unique name after 10 attempts (too many worktrees? use --name to specify)")

// adjectives for agent_id generation (~50 words).
var adjectives = []string{
	"swift", "brave", "calm", "bold", "keen",
	"warm", "cool", "wise", "fair", "fond",
	"quick", "bright", "dark", "light", "soft",
	"hard", "pure", "rare", "true", "free",
	"glad", "kind", "mild", "neat", "pale",
	"rich", "safe", "tall", "thin", "trim",
	"vast", "wild", "aged", "bare", "blue",
	"cold", "damp", "dear", "deep", "dull",
	"fast", "firm", "flat", "full", "gray",
	"high", "lean", "long", "loud", "sharp",
}

// animals for agent_id generation (~50 words).
var animals = []string{
	"fox", "owl", "elk", "bee", "ant",
	"jay", "cod", "eel", "bat", "ram",
	"cat", "dog", "pig", "cow", "hen",
	"rat", "ape", "yak", "koi", "gnu",
	"hog", "emu", "ray", "cub", "kit",
	"doe", "hart", "colt", "foal", "mare",
	"seal", "bear", "deer", "duck", "fawn",
	"goat", "hare", "hawk", "ibis", "lark",
	"lynx", "mole", "moth", "newt", "orca",
	"puma", "rook", "swan", "toad", "wolf",
}

// generateAgentID creates a unique adjective-animal identifier.
// existing is the list of current agent_ids and names to avoid collisions.
// Returns error after 10 failed attempts to find a unique ID.
func generateAgentID(existing []string) (string, error) {
	existingSet := make(map[string]bool, len(existing))
	for _, name := range existing {
		existingSet[name] = true
	}

	for range 10 {
		adjIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
		if err != nil {
			return "", fmt.Errorf("generating random adjective index: %w", err)
		}

		animalIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(animals))))
		if err != nil {
			return "", fmt.Errorf("generating random animal index: %w", err)
		}

		adj := adjectives[adjIdx.Int64()]
		animal := animals[animalIdx.Int64()]
		candidate := adj + "-" + animal

		if !existingSet[candidate] {
			return candidate, nil
		}
	}

	return "", ErrNameGenerationFailed
}

// getExistingNames returns all agent_ids and names from existing worktrees.
// Used for collision detection during agent_id generation.
func getExistingNames(worktrees []WorktreeInfo) []string {
	names := make([]string, 0, len(worktrees)*2)

	for _, wt := range worktrees {
		names = append(names, wt.AgentID)

		if wt.Name != wt.AgentID {
			names = append(names, wt.Name)
		}
	}

	return names
}
