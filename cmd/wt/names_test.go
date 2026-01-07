package main

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func Test_generateAgentID_Returns_Adjective_Animal_Format(t *testing.T) {
	t.Parallel()

	agentID, err := generateAgentID(nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	parts := strings.Split(agentID, "-")
	if len(parts) != 2 {
		t.Errorf("expected format 'adjective-animal', got %q", agentID)
	}

	// Verify first part is a valid adjective
	foundAdj := slices.Contains(adjectives, parts[0])

	if !foundAdj {
		t.Errorf("first part %q is not a valid adjective", parts[0])
	}

	// Verify second part is a valid animal
	foundAnimal := slices.Contains(animals, parts[1])

	if !foundAnimal {
		t.Errorf("second part %q is not a valid animal", parts[1])
	}
}

func Test_generateAgentID_Avoids_Existing_Names(t *testing.T) {
	t.Parallel()

	// Generate several IDs and ensure no duplicates
	existing := []string{}

	for range 20 {
		agentID, err := generateAgentID(existing)
		if err != nil {
			t.Fatalf("failed to generate agent_id: %v", err)
		}

		// Check it's not in existing
		for _, e := range existing {
			if agentID == e {
				t.Errorf("generated duplicate agent_id: %q", agentID)
			}
		}

		existing = append(existing, agentID)
	}
}

func Test_generateAgentID_Returns_Error_After_Exhausting_Retries(t *testing.T) {
	t.Parallel()

	// Create a list with all possible combinations
	allCombinations := make([]string, 0, len(adjectives)*len(animals))

	for _, adj := range adjectives {
		for _, animal := range animals {
			allCombinations = append(allCombinations, adj+"-"+animal)
		}
	}

	// Try to generate when all are taken
	_, err := generateAgentID(allCombinations)
	if err == nil {
		t.Fatal("expected error when all combinations exist, got nil")
	}

	if !errors.Is(err, ErrNameGenerationFailed) {
		t.Errorf("expected ErrNameGenerationFailed, got: %v", err)
	}
}

func Test_generateAgentID_Avoids_Collisions_With_Custom_Names(t *testing.T) {
	t.Parallel()

	// Existing includes both agent_ids and custom names
	existing := []string{"swift-fox", "my-custom-name", "brave-owl"}

	for range 50 {
		agentID, err := generateAgentID(existing)
		if err != nil {
			t.Fatalf("failed to generate agent_id: %v", err)
		}

		if agentID == "swift-fox" || agentID == "brave-owl" {
			t.Errorf("generated colliding agent_id: %q", agentID)
		}
	}
}

func Test_getExistingNames_Returns_Both_AgentID_And_Name(t *testing.T) {
	t.Parallel()

	worktrees := []WorktreeInfo{
		{Name: "swift-fox", AgentID: "swift-fox"},
		{Name: "custom-name", AgentID: "brave-owl"},
		{Name: "another", AgentID: "calm-deer"},
	}

	names := getExistingNames(worktrees)

	// Should have: swift-fox, brave-owl, custom-name, calm-deer, another
	// Note: swift-fox appears only once since Name == AgentID
	expected := map[string]bool{
		"swift-fox":   true,
		"brave-owl":   true,
		"custom-name": true,
		"calm-deer":   true,
		"another":     true,
	}

	if len(names) != len(expected) {
		t.Errorf("expected %d names, got %d: %v", len(expected), len(names), names)
	}

	for _, name := range names {
		if !expected[name] {
			t.Errorf("unexpected name in result: %q", name)
		}
	}
}

func Test_getExistingNames_Returns_Empty_For_No_Worktrees(t *testing.T) {
	t.Parallel()

	names := getExistingNames(nil)
	if len(names) != 0 {
		t.Errorf("expected empty slice, got: %v", names)
	}

	names = getExistingNames([]WorktreeInfo{})
	if len(names) != 0 {
		t.Errorf("expected empty slice, got: %v", names)
	}
}

func Test_WordLists_Have_Sufficient_Entries(t *testing.T) {
	t.Parallel()

	// Per SPEC: ~50 adjectives and ~50 animals for ~2500 combinations
	if len(adjectives) < 45 {
		t.Errorf("expected at least 45 adjectives, got %d", len(adjectives))
	}

	if len(animals) < 45 {
		t.Errorf("expected at least 45 animals, got %d", len(animals))
	}

	// Verify no duplicates in adjectives
	adjSet := make(map[string]bool)
	for _, adj := range adjectives {
		if adjSet[adj] {
			t.Errorf("duplicate adjective: %q", adj)
		}

		adjSet[adj] = true
	}

	// Verify no duplicates in animals
	animalSet := make(map[string]bool)
	for _, animal := range animals {
		if animalSet[animal] {
			t.Errorf("duplicate animal: %q", animal)
		}

		animalSet[animal] = true
	}
}

func Test_generateAgentID_Produces_Different_Results(t *testing.T) {
	t.Parallel()

	// Generate multiple IDs and check we get some variety
	results := make(map[string]bool)

	for range 50 {
		agentID, err := generateAgentID(nil)
		if err != nil {
			t.Fatalf("failed to generate agent_id: %v", err)
		}

		results[agentID] = true
	}

	// With 2500 combinations and 50 attempts, we should get at least 10 unique
	if len(results) < 10 {
		t.Errorf("expected at least 10 unique agent_ids from 50 attempts, got %d", len(results))
	}
}
