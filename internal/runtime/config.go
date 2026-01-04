package runtime

import (
	"github.com/contriboss/ore-light/internal/config"
	"github.com/contriboss/ore-light/internal/sources"
)

// Config is the shared ore-light configuration.
type Config = config.Config
type SourceConfig = sources.SourceConfig

// LoadConfig loads ore-light configuration from user and project config files.
func LoadConfig() (*Config, error) {
	return config.Load(), nil
}
