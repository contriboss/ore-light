package main

import (
	"github.com/contriboss/ore-light/internal/config"
	"github.com/contriboss/ore-light/internal/sources"
)

type SourceConfig = sources.SourceConfig

var appConfig = config.Load()
