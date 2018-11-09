package main

import (
	"net/url"
)

type config struct {
	Servers map[string]tfsConfig
	Proxy   proxy
}

type proxy struct {
	URL      string
	proxyURL *url.URL
}

type tfsConfig struct {
	tfs
	UseProxy bool
}
