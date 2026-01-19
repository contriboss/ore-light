package resolver_test

import (
	"github.com/contriboss/ore-light/internal/resolver"
	"testing"
)

func TestVersionSetContains(t *testing.T) {
	constraint := "<0.1.0 || >0.1.0"
	condition, err := resolver.NewSemverCondition(constraint)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	versionSet := condition.ToVersionSet()

	versions := []struct {
		version string
		expect  bool
	}{
		{"0.0.9", true},
		{"0.1.0", false},
		{"0.1.1", true},
		{"0.2.0", true},
	}

	for _, v := range versions {
		semverVer, err := resolver.NewSemverVersion(v.version)
		if err != nil {
			t.Fatalf("Error creating version %s: %v", v.version, err)
		}
		contained := versionSet.Contains(semverVer)
		if contained != v.expect {
			t.Errorf("VersionSet(%s) contains %s: got %v, want %v", constraint, v.version, contained, v.expect)
		}
	}
}
