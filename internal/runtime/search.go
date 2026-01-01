package runtime

// NewSearchSources returns a function that provides configured gem sources
func NewSearchSources(cfg *Config) func() []string {
	return func() []string {
		if cfg != nil && len(cfg.GemSources) > 0 {
			sources := make([]string, 0, len(cfg.GemSources))
			for _, src := range cfg.GemSources {
				sources = append(sources, src.URL)
			}
			return sources
		}

		return []string{"https://rubygems.org"}
	}
}
