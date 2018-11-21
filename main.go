package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	colorable "github.com/mattn/go-colorable"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

// Validate the connection
// Validate the permissions of PAT token
// Allow location of .toml file to be passed in.
// Add metrics for reporter
// Expose "ignoreHostedPools" externally. Should it be global or per project
// Improve logging (log lower level)
// Reformat the structure of tfsCollector to allow poolname to be captured
// Show error if not 200 is shown on http request
// Add "noAccessToken" flag for times when no auth is needed
// Show retry succeeded

func init() {

	log.SetFormatter(&log.TextFormatter{ForceColors: true})
	log.SetOutput(colorable.NewColorableStdout())
	//log.SetLevel(log.TraceLevel)
}

func main() {

	var pathToConfig = "config.toml"
	var port = 8080
	var endpoint = "/metrics"
	var ignoreHostedPools = true

	var proxyURL *url.URL

	configLogger := log.WithFields(log.Fields{
		"path": pathToConfig,
	})

	// Read config
	var c config
	if _, err := toml.DecodeFile(pathToConfig, &c); err != nil {
		configLogger.WithField("error", err).Error("Failed to decode configuration file")
		return
	}
	configLogger.Debug("Configuration file successfully decoded")

	// Validate config
	configValid := true
	for name, server := range c.Servers {

		//Check if access token exists as an Env Var
		envVar := strings.ToUpper(fmt.Sprintf("TFSEX_%v_ACCESSTOKEN", name))
		accessToken := os.Getenv(envVar)

		if accessToken != "" {
			configLogger.WithFields(log.Fields{"serverName": fmt.Sprintf("servers.%v", name), "envVar": envVar}).Info("Using AccessToken from environment variable")

			// AccessToken might already have been set from the config file. Just log we are going to override it.
			if server.AccessToken != "" {
				configLogger.WithFields(log.Fields{"serverName": fmt.Sprintf("servers.%v", name), "envVar": envVar}).Warning("AccessToken in config file will be overridden by AccessToken from environment variable")
			}

			// Assign EnvVar accessToken to the config object.
			server.AccessToken = accessToken
			c.Servers[name] = server
		} else {
			configLogger.WithFields(log.Fields{"serverName": fmt.Sprintf("servers.%v", name), "envVar": envVar}).Debug("Environment variable for AccessToken does not exist")
		}

		if server.AccessToken == "" {
			configLogger.WithFields(log.Fields{"serverName": fmt.Sprintf("servers.%v", name), "envVar": envVar}).Error("AccessToken not found in config file or environment variable")
			configValid = false
		}

		// Check that if a server has proxy set to true that the proxy table has been populated
		if server.UseProxy && c.Proxy.URL == "" {
			configLogger.WithField("serverName", fmt.Sprintf("servers.%v", name)).Error("UseProxy is true for but proxy url has not been set.")
			configValid = false
		}
	}

	// Safe even if c.Proxy.Url is empty
	proxyURL, err := url.Parse(c.Proxy.URL)
	if err != nil {
		configLogger.WithFields(log.Fields{"proxyURL": c.Proxy.URL, "error": err}).Error("proxyURL cannot be parsed as a URL")
		configValid = false
	}

	if configValid == false {
		configLogger.Fatal("Errors found within config")
		return
	}

	// Create and configure tfsCollector
	var tfsCollectors []*tfsCollector
	for name, server := range c.Servers {
		server.Name = name

		if server.UseProxy {
			log.WithFields(log.Fields{"server": server.Name, "serverAddress": server.Address}).Info("Proxy will be used")
			server.client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
		} else {
			server.client = &http.Client{}
		}

		tfsCollectors = append(tfsCollectors, newTFSCollector(server.tfs, ignoreHostedPools))
		log.WithFields(log.Fields{"server": server.Name, "serverAddress": server.Address}).Info("Metrics collector created")
	}

	// Add each tfsCollector to the register so they get called when Prometheus scrapes.
	var reg = prometheus.NewRegistry()
	for _, tc := range tfsCollectors {
		prometheus.WrapRegistererWith(prometheus.Labels{"name": tc.tfs.Name}, reg).MustRegister(tc)
	}

	http.Handle(endpoint, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	log.Info("Serving metrics at " + endpoint + " on port: " + strconv.Itoa(port))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}
