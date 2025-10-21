package commands

import (
	"flag"
	"fmt"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/resolver"
)

// RunAdd implements the ore add command
func RunAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	version := fs.String("version", "", "Version constraint (e.g., ~> 8.0)")
	group := fs.String("group", "", "Group to add gem to")
	github := fs.String("github", "", "GitHub repository (user/repo)")
	git := fs.String("git", "", "Git repository URL")
	branch := fs.String("branch", "", "Git branch")
	tag := fs.String("tag", "", "Git tag")
	ref := fs.String("ref", "", "Git reference")
	path := fs.String("path", "", "Local path to gem")
	requireFlag := fs.Bool("require", true, "Whether to require the gem")
	lock := fs.Bool("lock", false, "Automatically resolve and update Gemfile.lock")
	verbose := fs.Bool("v", false, "Enable verbose output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	gems := fs.Args()
	if len(gems) == 0 {
		return fmt.Errorf("at least one gem name is required")
	}

	// Find Gemfile
	paths, err := lockfile.FindGemfiles()
	if err != nil {
		return fmt.Errorf("failed to find Gemfile: %w", err)
	}

	if *verbose {
		fmt.Printf("📝 Adding gems to %s...\n", paths.GetGemfileName())
	}

	// Process each gem
	for _, gemName := range gems {
		dep := gemfile.GemDependency{
			Name: gemName,
		}

		// Add group
		if *group != "" {
			dep.Groups = []string{*group}
		}

		// Add version constraints
		if *version != "" {
			dep.Constraints = []string{*version}
		}

		// Add source information
		if *github != "" || *git != "" || *path != "" {
			source := &gemfile.Source{}
			if *github != "" {
				source.Type = "git"
				source.URL = fmt.Sprintf("https://github.com/%s.git", *github)
			} else if *git != "" {
				source.Type = "git"
				source.URL = *git
			} else if *path != "" {
				source.Type = "path"
				source.URL = *path
			}

			if *branch != "" {
				source.Branch = *branch
			}
			if *tag != "" {
				source.Tag = *tag
			}
			if *ref != "" {
				source.Ref = *ref
			}

			dep.Source = source
		}

		// Add require option
		if !*requireFlag {
			requireFalse := "false"
			dep.Require = &requireFalse
		}

		// Add gem to Gemfile using gemfile-go writer
		if err := gemfile.AddGemToFile(paths.Gemfile, &dep); err != nil {
			return fmt.Errorf("failed to add gem %s: %w", gemName, err)
		}

		if *verbose {
			fmt.Printf("✅ Added gem: %s\n", gemName)
		}
	}

	fmt.Println("✨ Gems added successfully")

	// Optionally resolve and update lockfile
	if *lock {
		if *verbose {
			fmt.Println("🔒 Resolving dependencies and updating lockfile...")
		}
		if err := resolver.GenerateLockfile(paths.Gemfile); err != nil {
			return fmt.Errorf("failed to generate lockfile: %w", err)
		}
		fmt.Println("💡 Run 'ore install' to fetch the new gems")
	} else {
		fmt.Println("💡 Run 'bundle lock' (or use --lock flag) to update Gemfile.lock, then 'ore install'")
	}

	return nil
}
