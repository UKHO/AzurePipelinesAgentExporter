# Azure Pipelines Agents Prometheus Exporter

Prometheus exporter for Azure Pipelines/Azure DevOps Server/TFS private agents. Exports metrics helpful when running a large estate of private agents across numerous queues.

- Works with Azure Pipelines, Azure DevOps Server and TFS 2018
- Supports scraping multiple instances from one exporter
- Basic support for corprate firewall
- Supports access tokens via environment variables
- Configured via TOML


## Configuration

The exporter is configured with a [TOML](https://github.com/toml-lang/toml) file when the exporter is started through the `--config` flag.

By default exposes metrics at `:8080/metrics`

Each server being scraped has it configuration "block". These require a unique name. The name is added as labels to aid diagnosing problems between servers. If the name is changed then metrics will be labeled with the new name, this can cause problems.

```toml
[servers]
    [servers.{name}]
```

Access tokens should be configured through environment variables. The name of the variables must be - ``TFSEX_{name}_ACCESSTOKEN``. Access tokens can also be configured via the configuration file(see [FullConfiguration](#FullConfiguration))

### Basic Configuration

```toml
[servers]

    # On Premises TFS server
    [servers.tfs] # Server "name" is TFS. This can be any string.
    address = "http://tfs:8080/tfs"
    defaultCollection = "dc"

    # Azure Pipelines
    [servers.azuredevops]
    address = "https://dev.azure.com/devorg"

    # Azure Pipelines
    [servers.OtherAzureDevOpsInstance]
    address = "https://dev.azure.com/devorg2"
```

### Configuration with proxy

```toml
[servers]

    # Azure Pipelines
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
    # As access token isn't specified, an environemnt token called TFSEX_TFSInstance_ACCESSTOKEN needs to have been created

[proxy]
    url = "http://proxy.devorg.com:9191"
```

## Tips

Set Promethus timeout to be larger than 10 seconds as scrapes can sometimes take longer than that.

## Metrics Exposed

- tfs_build_agents_total
  - Gauge of the total installed build agents. Has labels of `"enabled", "status", "pool" "name"`
- tfs_build_agents_total_scrape_duration_seconds
  - Gauge of duration of time it took to scrape total of installed build agents. Has labels of `"name"`
