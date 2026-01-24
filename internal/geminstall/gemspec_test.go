package geminstall

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGenerateGenericGemspec_Fidelity(t *testing.T) {
	metadataYAML := []byte(`
name: test-gem
version: 1.0.0
authors: ["Author A", "Author B"]
date: 2025-01-01 00:00:00.000000000 Z
specification_version: 4
rubgygems_version: 3.5.0
dependencies:
  - name: dep1
    type: :runtime
    requirement:
      requirements: [[">=", "1.0"]]
  - name: dep2
    type: :development
    requirement:
      requirements: [[">=", "2.0"]]
`)

	cleanedYAML := stripRubyYAMLTags(metadataYAML)
	var rawData map[string]interface{}
	if err := yaml.Unmarshal(cleanedYAML, &rawData); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Simulate the pipeline
	filterAndNormalize(rawData)

	rubyCode, err := generateGenericGemspec(rawData)
	if err != nil {
		t.Fatalf("Failed to generate gemspec: %v", err)
	}

	// Assertions
	checks := []struct {
		desc     string
		contains string
		exclude  bool
	}{
		{"Has installed_by_version", "s.installed_by_version =", false}, // It should be added if missing, or present
		{"Removes date", "s.date =", true},
		{"Removes specification_version", "s.specification_version =", true},
		{"Converts runtime dep to add_dependency", `s.add_dependency("dep1".freeze, ">= 1.0".freeze)`, false},
		{"Converts dev dep to add_development_dependency", `s.add_development_dependency("dep2".freeze, ">= 2.0".freeze)`, false},
		{"Handles authors array", `s.authors = ["Author A".freeze, "Author B".freeze].freeze`, false},
	}

	for _, check := range checks {
		has := strings.Contains(rubyCode, check.contains)
		if check.exclude {
			if has {
				t.Errorf("Expected gemspec NOT to contain %q (%s), but it did.", check.contains, check.desc)
			}
		} else {
			if !has {
				t.Errorf("Expected gemspec to contain %q (%s), but it didn't.\nGenerated Code:\n%s", check.contains, check.desc, rubyCode)
			}
		}
	}
}
