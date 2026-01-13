package resolver

import (
	"strings"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/pubgrub-go"
)

func constraintsFromCondition(cond pubgrub.Condition) []string {
	if cond == nil {
		return nil
	}
	str := strings.TrimSpace(cond.String())
	if str == "" || str == "*" || str == ">= 0" {
		return nil
	}
	return splitConstraintString(str)
}

func lockfileDependencyFromTerm(term pubgrub.Term) lockfile.Dependency {
	return lockfile.Dependency{
		Name:        term.Name.Value(),
		Constraints: constraintsFromCondition(term.Condition),
	}
}

func dependenciesFromCompactMap(deps map[string]string) []lockfile.Dependency {
	if len(deps) == 0 {
		return nil
	}

	out := make([]lockfile.Dependency, 0, len(deps))
	for name, constraint := range deps {
		out = append(out, lockfile.Dependency{
			Name:        name,
			Constraints: splitConstraintString(constraint),
		})
	}

	return out
}

func splitConstraintString(constraint string) []string {
	clean := strings.TrimSpace(constraint)
	if clean == "" || clean == ">= 0" {
		return nil
	}

	// Compact index can use '&' for AND; normalize to commas.
	clean = strings.ReplaceAll(clean, "&", ",")

	parts := strings.Split(clean, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func conditionFromConstraints(constraints []string) (pubgrub.Condition, bool) {
	if len(constraints) == 0 {
		return nil, false
	}

	constraintStr := strings.TrimSpace(strings.Join(constraints, ", "))
	if constraintStr == "" || constraintStr == "*" || constraintStr == ">= 0" {
		return nil, false
	}

	semverCond, err := NewSemverCondition(constraintStr)
	if err != nil {
		return nil, false
	}
	return semverCond, true
}
