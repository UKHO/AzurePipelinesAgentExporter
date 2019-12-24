package main

import (
	"net/url"

	"./azdo"
)

var (
	portDefault     = 8080
	endpointDefault = "/metrics"
)

type config struct {
	Servers  map[string]azDoConfig
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

type azDoConfig struct {
	azdo.AzDoClient
	UseProxy bool
}
