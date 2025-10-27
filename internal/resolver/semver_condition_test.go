package resolver

import (
	"testing"

	"github.com/contriboss/pubgrub-go"
)

func TestSemverCondition(t *testing.T) {
	tests := []struct {
		constraint string
		version    string
		satisfied  bool
	}{
		// Ruby tilde-arrow constraints
		{"~> 2.1.0", "2.1.0", true},
		{"~> 2.1.0", "2.1.5", true},
		{"~> 2.1.0", "2.2.0", false},

		// Range constraints
		{">= 2.0.0, < 3.0.0", "2.5.0", true},
		{">= 2.0.0, < 3.0.0", "1.9.0", false},
		{">= 2.0.0, < 3.0.0", "3.0.0", false},

		// Multi-segment Ruby versions
		{">= 3.3.0.2", "3.3.0.2", true},
		{">= 3.3.0.2", "3.3.0.1", false},
		{"~> 3.3.0.2", "3.3.0.5", true},
		{"~> 3.3.0.2", "3.3.1.0", false},

		// Equality and inequality semantics
		{"= 1.0", "1.0.0", true},
		{"!= 1.0.0", "1.0.0", false},
		{"!= 1.0.0", "1.0.1", true},

		// Pre-release ordering
		{"< 1.0.0", "1.0.0.beta1", true},
		{">= 1.0.0", "1.0.0.beta1", false},
	}

	for _, test := range tests {
		t.Run(test.constraint+"_"+test.version, func(t *testing.T) {
			condition, err := NewSemverCondition(test.constraint)
			if err != nil {
				t.Fatalf("Failed to create condition: %v", err)
			}

			version, err := NewSemverVersion(test.version)
			if err != nil {
				t.Fatalf("Failed to create version: %v", err)
			}

			satisfied := condition.Satisfies(version)
			if satisfied != test.satisfied {
				t.Errorf("Expected %s to satisfy %s: %t, got %t",
					test.version, test.constraint, test.satisfied, satisfied)
			}
		})
	}
}

func TestPubGrubWithSemver(t *testing.T) {
	// Create an in-memory source for testing
	source := &pubgrub.InMemorySource{}

	// Add rails versions
	rails800, _ := NewSemverVersion("8.0.0")
	rails801, _ := NewSemverVersion("8.0.1")

	// Add rack versions
	rack220, _ := NewSemverVersion("2.2.0")
	rack225, _ := NewSemverVersion("2.2.5")
	rack230, _ := NewSemverVersion("2.3.0")

	// Rails 8.0.0 depends on rack ~> 2.2.0
	rackConstraint, _ := NewSemverCondition("~> 2.2.0")
	source.AddPackage(
		pubgrub.MakeName("rails"),
		rails800,
		[]pubgrub.Term{
			pubgrub.NewTerm(pubgrub.MakeName("rack"), rackConstraint),
		},
	)

	// Rails 8.0.1 depends on rack ~> 2.2.0
	source.AddPackage(
		pubgrub.MakeName("rails"),
		rails801,
		[]pubgrub.Term{
			pubgrub.NewTerm(pubgrub.MakeName("rack"), rackConstraint),
		},
	)

	// Add rack versions (no dependencies)
	source.AddPackage(pubgrub.MakeName("rack"), rack220, []pubgrub.Term{})
	source.AddPackage(pubgrub.MakeName("rack"), rack225, []pubgrub.Term{})
	source.AddPackage(pubgrub.MakeName("rack"), rack230, []pubgrub.Term{})

	// Create root source and solver using new API
	root := pubgrub.NewRootSource()
	railsConstraint, _ := NewSemverCondition("~> 8.0.0")
	root.AddPackage(pubgrub.MakeName("rails"), railsConstraint)

	solver := pubgrub.NewSolver(root, source)
	solution, err := solver.Solve(root.Term())
	if err != nil {
		t.Fatalf("Failed to solve: %v", err)
	}

	t.Logf("Solution: %v", solution)

	// Verify we got a valid solution (rails + rack, no root)
	if len(solution) != 3 {
		t.Errorf("Expected 3 packages in solution (root + rails + rack), got %d", len(solution))
	}

	// Check that rails and rack are in the solution
	var railsVersion, rackVersion string
	for _, pkg := range solution {
		if pkg.Name.Value() == "rails" {
			railsVersion = pkg.Version.String()
		} else if pkg.Name.Value() == "rack" {
			rackVersion = pkg.Version.String()
		}
	}

	if railsVersion == "" {
		t.Error("Rails not found in solution")
	}
	if rackVersion == "" {
		t.Error("Rack not found in solution")
	}

	t.Logf("Selected: rails %s, rack %s", railsVersion, rackVersion)

	// Verify the selected versions satisfy the original constraints
	selectedRails, _ := NewSemverVersion(railsVersion)
	selectedRack, _ := NewSemverVersion(rackVersion)

	if !railsConstraint.Satisfies(selectedRails) {
		t.Errorf("Selected rails version %s does not satisfy constraint %s", railsVersion, railsConstraint.String())
	}

	if !rackConstraint.Satisfies(selectedRack) {
		t.Errorf("Selected rack version %s does not satisfy constraint %s", rackVersion, rackConstraint.String())
	}
}
