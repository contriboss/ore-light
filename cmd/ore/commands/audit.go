package commands

import (
	"flag"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/contriboss/ore-light/internal/audit"
)

// RunAudit implements the ore audit command
func RunAudit(args []string) error {
	if len(args) > 0 && args[0] == "licenses" {
		return runAuditLicenses(args[1:])
	}
	if len(args) > 0 && args[0] == "update" {
		return runAuditUpdate(args[1:])
	}

	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", "", "Path to Gemfile (used to derive lockfile path)")
	lockfilePath := fs.String("lockfile", "", "Path to Gemfile.lock")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve effective lockfile path
	effectiveLockfilePath := *lockfilePath
	if effectiveLockfilePath == "" {
		if *gemfilePath != "" {
			// If -gemfile is provided, use it to derive lockfile path
			effectiveLockfilePath = *gemfilePath + ".lock"
		} else {
			effectiveLockfilePath = defaultLockfilePath()
		}
	}

	// Load lockfile
	parsed, err := loadLockfile(effectiveLockfilePath)
	if err != nil {
		return err
	}

	// Initialize database
	db, err := audit.NewDatabase("")
	if err != nil {
		return err
	}

	if !db.Exists() {
		fmt.Println("Advisory database not found. Run `ore audit update` to download it.")
		return fmt.Errorf("advisory database not found")
	}

	// Create scanner and scan
	scanner := audit.NewScanner(db)
	result, err := scanner.ScanWithReport(parsed.GemSpecs)
	if err != nil {
		return err
	}

	// Print results
	printAuditResults(result)

	if result.HasVulnerabilities() {
		return fmt.Errorf("vulnerabilities found")
	}

	return nil
}

func runAuditUpdate(args []string) error {
	fs := flag.NewFlagSet("audit update", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := audit.NewDatabase("")
	if err != nil {
		return err
	}

	return db.Update()
}

func runAuditLicenses(args []string) error {
	fs := flag.NewFlagSet("audit licenses", flag.ContinueOnError)
	vendorDir := fs.String("vendor", defaultVendorDir(), "Path to installed gems")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Scan for licenses
	report, err := audit.ScanLicenses(*vendorDir)
	if err != nil {
		return err
	}

	// Print the report
	audit.PrintLicenseReport(report)

	return nil
}

func printAuditResults(result *audit.ScanResult) {
	if !result.HasVulnerabilities() {
		successStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")). // Green
			Bold(true)
		fmt.Println(successStyle.Render("✓ No vulnerabilities found."))
		return
	}

	// Header style
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")). // Red
		Bold(true)

	fmt.Printf("%s\n\n", headerStyle.Render(fmt.Sprintf("Found %d vulnerabilities in %d gems:", result.VulnerabilityCount(), result.VulnerableGemCount())))

	for _, vuln := range result.Vulnerabilities {
		printVulnerability(vuln)
	}
}

func printVulnerability(vuln audit.Vulnerability) {
	// Styles
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")). // Gray
		Bold(true)

	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("13")). // Magenta
		Bold(true)

	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11")) // Yellow

	advisoryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")) // Blue

	urlStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("14")). // Cyan
		Underline(true)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")) // White

	solutionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")) // Green

	// Get severity color
	severityStyle := lipgloss.NewStyle()
	severity := vuln.Advisory.Severity()
	switch strings.ToLower(severity) {
	case "critical":
		severityStyle = severityStyle.Foreground(lipgloss.Color("9")).Bold(true) // Red
	case "high":
		severityStyle = severityStyle.Foreground(lipgloss.Color("208")) // Orange
	case "medium":
		severityStyle = severityStyle.Foreground(lipgloss.Color("11")) // Yellow
	case "low":
		severityStyle = severityStyle.Foreground(lipgloss.Color("12")) // Blue
	default:
		severityStyle = severityStyle.Foreground(lipgloss.Color("8")) // Gray
	}

	// Print vulnerability info
	fmt.Printf("%s %s\n", labelStyle.Render("Name:"), nameStyle.Render(vuln.Gem.Name))
	fmt.Printf("%s %s\n", labelStyle.Render("Version:"), versionStyle.Render(vuln.Gem.Version))
	fmt.Printf("%s %s\n", labelStyle.Render("Advisory:"), advisoryStyle.Render(vuln.Advisory.ID()))

	if severity != "Unknown" {
		fmt.Printf("%s %s\n", labelStyle.Render("Criticality:"), severityStyle.Render(severity))
	}

	fmt.Printf("%s %s\n", labelStyle.Render("URL:"), urlStyle.Render(vuln.Advisory.URL))
	fmt.Printf("%s %s\n", labelStyle.Render("Title:"), titleStyle.Render(vuln.Advisory.Title))

	if len(vuln.Advisory.PatchedVersions) > 0 {
		fmt.Printf("%s %s\n", labelStyle.Render("Solution:"), solutionStyle.Render("update to "+strings.Join(vuln.Advisory.PatchedVersions, " or ")))
	}

	fmt.Println()
}
