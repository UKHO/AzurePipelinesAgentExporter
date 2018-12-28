package main

import (
	"net/url"
)

var (
	portDefault     = 8080
	endpointDefault = "/metrics"
)

type config struct {
	Servers  map[string]tfsConfig
	Proxy    proxy
	Exporter exporter
}

type exporter struct {
	Port     int
	Endpoint string
}

type proxy struct {
	URL      string
	proxyURL *url.URL
}

type tfsConfig struct {
	tfs
	UseProxy bool
}
