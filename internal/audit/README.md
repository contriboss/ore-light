# Ore Audit Package

Security vulnerability scanning for Ruby gems, compatible with bundler-audit.

## About

This is a **simplified implementation** extracted from the original Ore project (ore_reference).
It provides familiar bundler-audit compatibility using a Go implementation, allowing vulnerability
scanning without requiring Ruby to be installed.

## Compatibility with bundler-audit

Uses the same vulnerability database and behavior as bundler-audit for familiar workflows.

### Database

- **ruby-advisory-db**: https://github.com/rubysec/ruby-advisory-db
- Stored at: `~/.local/share/ruby-advisory-db` (same as bundler-audit)
- Compatible with `BUNDLER_AUDIT_DB` environment variable

### Key Differences

| Feature | bundler-audit | ore-light audit |
|---------|--------------|-----------------|
| Language | Ruby | Go (native) |
| Requires Ruby | Yes | No |
| Database format | YAML | YAML (same) |
| Database location | Same | Same |
| Advisory matching | Ruby gems | Go implementation |

## Feature Set

This is a **limited feature set** extracted from the full Ore implementation to provide:
- Basic vulnerability scanning
- bundler-audit database compatibility
- Standalone operation (no Ruby required)

For users familiar with bundler-audit, this provides the same workflow and database.

## Usage

```go
// Update database
db, _ := audit.NewDatabase("")
db.Update()

// Scan gems
scanner := audit.NewScanner(db)
results := scanner.Scan(gemSpecs)
```

## Architecture

- `advisory.go` - Advisory data structures
- `database.go` - Database cloning and loading
- `scanner.go` - Vulnerability scanning logic
- `version.go` - Version constraint matching
