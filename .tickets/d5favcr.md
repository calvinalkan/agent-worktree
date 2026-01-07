---
schema_version: 1
id: d5favcr
status: in_progress
blocked-by: [d5fav78]
created: 2026-01-07T19:02:11Z
type: task
priority: 1
---
# Implement name generation (adjective-animal word lists)

## Overview
Implement the agent_id generation system using adjective-animal word combinations.

## Background & Rationale
Per SPEC.md, each worktree gets an auto-generated agent_id in the format `<adjective>-<animal>` (e.g., swift-fox, brave-owl). This provides:
- Human-memorable identifiers
- ~2,500 unique combinations (50x50)
- Collision detection with retry logic

The name serves as the default worktree directory and branch name unless overridden with --name.

## Current State
No word lists or generation logic exists.

## Implementation Details

### Word Lists (add to run.go or new names.go)
```go
// adjectives for agent_id generation (~50 words).
var adjectives = []string{
    "swift", "brave", "calm", "bold", "keen",
    "warm", "cool", "wise", "fair", "fond",
    "quick", "bright", "dark", "light", "soft",
    "hard", "pure", "rare", "true", "free",
    "glad", "kind", "mild", "neat", "pale",
    "rich", "safe", "tall", "thin", "trim",
    "vast", "warm", "wild", "aged", "bare",
    "blue", "cold", "damp", "dear", "deep",
    "dull", "fast", "firm", "flat", "full",
    "gray", "high", "lean", "long", "loud",
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
```

### Generation Function
```go
// generateAgentID creates a unique adjective-animal identifier.
// existing is the list of current agent_ids and names to avoid collisions.
// Returns error after 10 failed attempts to find a unique ID.
func generateAgentID(existing []string) (string, error) {
    existingSet := make(map[string]bool, len(existing))
    for _, e := range existing {
        existingSet[e] = true
    }
    
    for i := 0; i < 10; i++ {
        adj := adjectives[rand.Intn(len(adjectives))]
        animal := animals[rand.Intn(len(animals))]
        candidate := adj + "-" + animal
        
        if !existingSet[candidate] {
            return candidate, nil
        }
    }
    
    return "", errors.New("failed to generate unique agent_id after 10 attempts")
}
```

### Collision Check Helper
```go
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
```

## Design Decisions
- 50x50 word lists = 2,500 combinations (per SPEC)
- Short words (3-5 chars) for easy typing
- Common, easy-to-spell words
- 10 retry limit prevents infinite loops on near-full namespaces
- Check both agent_ids AND names for collisions (user might use --name with an adjective-animal combo)

## Acceptance Criteria
- Word lists contain ~50 adjectives and ~50 animals
- generateAgentID returns unique `adjective-animal` format
- Collision detection against existing agent_ids AND names
- Error after 10 failed attempts
- Uses math/rand (seeded) for randomness

## Testing
- Test generation produces valid format
- Test collision avoidance
- Test error after exhausting retries
