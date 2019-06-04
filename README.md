# Azure Pipelines Agents Prometheus Exporter

Prometheus exporter for Azure Pipelines/Azure DevOps Server/TFS private agents. Exports metrics helpful when running a large estate of private agents across numerous queues.

- Works with Azure Pipelines, Azure DevOps Server and TFS 2018
- Supports scraping multiple servers from one exporter
- Basic support for corprate firewalls
- Supports access tokens through environment variables
- Configured via TOML

## Configuration

The exporter is configured by a [TOML](https://github.com/toml-lang/toml) file. This is passed to the exporter when it starts by using the `--config` flag.

By default it exposes metrics at `:8080/metrics`

Each server being scraped has it's own configuration "block". These each require a unique name. The name is added as a label to aid diagnosing problems between servers. If the name is changed then metrics will be labeled with the new name, this can cause problems.

```toml
[servers]
    [servers.{name}]
```

Access tokens should be configured through environment variables. The name of the environment variable must be - ``TFSEX_{name}_ACCESSTOKEN``. Access tokens can also be configured through the configuration file (see [Full Configuration](#Full-Configuration)) but this behaviour is discouraged.

### Basic Configuration

```toml
[servers]

    # As the access token isn't specfied in the configuration file, the exporter expects the access token to be in an environment variable.

    # On Premises TFS server
    [servers.tfs] # Server "name" is tfs.
    address = "http://tfs:8080/tfs"
    defaultCollection = "dc"

    # Azure Pipelines
    [servers.azuredevops] # Server "name" is azuredevops.
    address = "https://dev.azure.com/devorg"

    # Azure Pipelines
    [servers.OtherAzureDevOpsInstance] # Server "name" is OtherAzureDevOpsInstance
    address = "https://dev.azure.com/devorg2"
```

### Configuration with proxy

```toml
[servers]

    [servers.azuredevops]
    address = "https://dev.azure.com/devorg"
    useProxy = true

[proxy]
    url = "http://proxy.devorg.com:9191"
```

### Full Configuration

```toml
[exporter]
    port = 9595
    endpoint = "/tfsmetrics"

[servers]

    # Azure Pipelines
    [servers.azuredevops]
    address = "https://dev.azure.com/devorg"
    useProxy = true
    accessToken = "thisisamadeupaccesstoken"

    [servers.TFSInstance]
    address = "http://tfs:8080/tfs"
    defaultCollection = "dc"
    # As access token isn't specified, an environemnt token called TFSEX_TFSInstance_ACCESSTOKEN needs to have been created and populated

[proxy]
    url = "http://proxy.devorg.com:9191"
```

## Tips

Set the promethus scrape timeout to be larger than 10 seconds as scrapes can sometimes be longer 10s.

## Metrics Exposed

- tfs_build_agents_total
  - Gauge of the total installed build agents. Has labels of `"enabled", "status", "pool" "name"`
- tfs_build_agents_total_scrape_duration_seconds
  - Gauge of duration of time it took to scrape total of installed build agents. Has labels of `"name"`
