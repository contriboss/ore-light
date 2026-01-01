package commands

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/contriboss/gemfile-go/lockfile"
)

// RunExec implements the ore exec command
func RunExec(args []string, buildEnv func(vendorDir string, specs []lockfile.GemSpec) ([]string, error)) error {
	fs := flag.NewFlagSet("exec", flag.ContinueOnError)
	lockfilePath := fs.String("lockfile", defaultLockfilePath(), "Path to Gemfile.lock")
	vendorDir := fs.String("vendor", defaultVendorDir(), "Path to installed gems (created by ore install)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cmdArgs := fs.Args()
	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command provided; usage: ore exec [options] -- <command> [args...]")
	}

	gems, err := loadGemSpecs(*lockfilePath)
	if err != nil {
		return err
	}

	env, err := buildEnv(*vendorDir, gems)
	if err != nil {
		return err
	}

	// When using system gems, run command directly (not via bundle exec)
	// Bundler's auto-load in Ruby 3.4+ handles gem activation automatically
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
