package audit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// DatabaseURL is the Git URL for ruby-advisory-db
	DatabaseURL = "https://github.com/rubysec/ruby-advisory-db.git"
)

// Database represents the ruby-advisory-db
type Database struct {
	Path string
}

// DefaultDatabasePath returns the default path for the advisory database
func DefaultDatabasePath() (string, error) {
	// Check environment variable first
	if path := os.Getenv("ORE_AUDIT_DB"); path != "" {
		return path, nil
	}
	if path := os.Getenv("BUNDLER_AUDIT_DB"); path != "" {
		return path, nil
	}

	// Default to ~/.local/share/ruby-advisory-db
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "share", "ruby-advisory-db"), nil
}

// NewDatabase creates a new database instance
func NewDatabase(path string) (*Database, error) {
	if path == "" {
		var err error
		path, err = DefaultDatabasePath()
		if err != nil {
			return nil, err
		}
	}

	return &Database{Path: path}, nil
}

// Exists checks if the database has been downloaded
func (db *Database) Exists() bool {
	gemsDir := filepath.Join(db.Path, "gems")
	info, err := os.Stat(gemsDir)
	return err == nil && info.IsDir()
}

// Update clones or updates the advisory database
func (db *Database) Update() error {
	if !db.Exists() {
		// Clone the database
		fmt.Printf("Cloning ruby-advisory-db to %s...\n", db.Path)

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(db.Path), 0o755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		cmd := exec.Command("git", "clone", "--depth", "1", DatabaseURL, db.Path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone advisory database: %w", err)
		}

		fmt.Println("Advisory database cloned successfully.")
		return nil
	}

	// Update existing database
	fmt.Println("Updating ruby-advisory-db...")

	cmd := exec.Command("git", "-C", db.Path, "pull", "--ff-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update advisory database: %w", err)
	}

	fmt.Println("Advisory database updated successfully.")
	return nil
}

// LoadAdvisories loads all advisories for a specific gem
func (db *Database) LoadAdvisories(gemName string) ([]Advisory, error) {
	gemDir := filepath.Join(db.Path, "gems", gemName)

	// Check if gem has advisories
	if _, err := os.Stat(gemDir); os.IsNotExist(err) {
		return nil, nil // No advisories for this gem
	}

	entries, err := os.ReadDir(gemDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read gem advisories: %w", err)
	}

	var advisories []Advisory
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}

		advisoryPath := filepath.Join(gemDir, entry.Name())
		advisory, err := db.loadAdvisory(advisoryPath)
		if err != nil {
			// Log warning but continue
			fmt.Fprintf(os.Stderr, "Warning: failed to load %s: %v\n", advisoryPath, err)
			continue
		}

		advisories = append(advisories, advisory)
	}

	return advisories, nil
}

// loadAdvisory loads a single advisory from a YAML file
func (db *Database) loadAdvisory(path string) (Advisory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Advisory{}, err
	}

	var advisory Advisory
	if err := yaml.Unmarshal(data, &advisory); err != nil {
		return Advisory{}, err
	}

	return advisory, nil
}

// Size returns the number of gems with advisories
func (db *Database) Size() (int, error) {
	gemsDir := filepath.Join(db.Path, "gems")
	entries, err := os.ReadDir(gemsDir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}

	return count, nil
}
