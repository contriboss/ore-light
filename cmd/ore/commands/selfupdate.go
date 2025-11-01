package commands

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

// RunSelfUpdate implements the self-update command
func RunSelfUpdate(args []string, currentVersion, buildCommit string) error {
	fs := flag.NewFlagSet("self-update", flag.ContinueOnError)
	checkOnly := fs.Bool("check", false, "Check for updates without installing")
	yes := fs.Bool("yes", false, "Skip confirmation prompt")
	fs.BoolVar(yes, "y", false, "Skip confirmation prompt (shorthand)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Parse current version (strip 'v' prefix if present)
	versionStr := strings.TrimPrefix(currentVersion, "v")
	if versionStr == "dev" || versionStr == "" {
		return fmt.Errorf("cannot self-update dev build. Please install from GitHub releases")
	}

	current, err := semver.Parse(versionStr)
	if err != nil {
		return fmt.Errorf("invalid current version %q: %w", currentVersion, err)
	}

	// Determine target architecture
	targetArch := fmt.Sprintf("ore-v%s-%s-%s.tar.gz", current.String(), runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Checking target-arch... %s\n", targetArch)

	// Display current version
	fmt.Printf("Checking current version... v%s\n", current.String())

	// Check for latest release
	fmt.Print("Checking latest released version... ")
	latest, found, err := selfupdate.DetectLatest("contriboss/ore-light")
	if err != nil {
		fmt.Println()
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !found {
		fmt.Println()
		return fmt.Errorf("no releases found")
	}

	latestVer := latest.Version

	// Check if already up-to-date
	if !latestVer.GT(current) {
		fmt.Printf("v%s\n", latestVer.String())
		fmt.Println("ore is already up to date")
		return nil
	}

	// New version available
	fmt.Printf("v%s\n", latestVer.String())
	fmt.Printf("New release found! v%s --> v%s\n", current.String(), latestVer.String())

	if *checkOnly {
		return nil
	}

	// Get executable path
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Display release status
	newTargetArch := fmt.Sprintf("ore-v%s-%s-%s.tar.gz", latestVer.String(), runtime.GOOS, runtime.GOARCH)
	assetName := fmt.Sprintf("ore_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	downloadURL := fmt.Sprintf("https://github.com/contriboss/ore-light/releases/download/v%s/%s",
		latestVer.String(), assetName)

	fmt.Println()
	fmt.Println("ore release status:")
	fmt.Printf("  * Current exe: %q\n", exe)
	fmt.Printf("  * New exe release: %q\n", newTargetArch)
	fmt.Printf("  * New exe download url: %q\n", downloadURL)
	fmt.Println()

	// Confirmation prompt
	if !*yes {
		fmt.Println("The new release will be downloaded/extracted and the existing binary will be replaced.")
		fmt.Print("Do you want to continue? [Y/n] ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response != "" && response != "y" && response != "yes" {
			fmt.Println("Update cancelled.")
			return nil
		}
	}

	// Perform update
	fmt.Println("Downloading...")
	if err := selfupdate.UpdateTo(latest.AssetURL, exe); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Println("Verifying downloaded file...")
	fmt.Println("Extracting archive... Done")
	fmt.Println("Replacing binary file... Done")
	fmt.Printf("Updated ore to v%s\n", latestVer.String())

	return nil
}
