package main

import (
	rubyext "github.com/contriboss/ruby-extension-go"
	rubygemsclient "github.com/contriboss/rubygems-client-go"
)

// NOTE: These integration helpers keep the ore (light edition) CLI linked to
// the extracted modules from the reference implementation. As the MVP grows we
// will replace this with real usage (provider selection, extension builds, etc.)
var (
	_ = rubygemsclient.NewClient
	_ = rubyext.NewBuilderFactory
)
