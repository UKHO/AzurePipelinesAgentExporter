# Azure Pipelines Agents Prometheus Exporter

A Prometheus exporter for Azure DevOps/Azure DevOps Server self hosted agents. Exposes metrics helpful for running agents across numerous queues.

- Works with Azure DevOps and Azure DevOps Server. Untested support for TFS 2018
- Scrapes multiple servers from one exporter
- Basic support for corporate firewalls
- Configured via TOML

![Graph of metrics displayed in Grafana](/SampleGraphs.PNG)

## Docker Quickstart

Create a [Personal Access Token](https://docs.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops&tabs=preview-page) with the following permissions:

- Agent Pools (Read)
- Build (Read)
- Release (Read)

Create a `config.toml` file with this configuration:

```toml
[servers]
    [servers.azdo]
    address = "https://dev.azure.com/your_org_goes_here"
```

Start the container while binding the `config.toml` created above

```bash
docker run \
  -v $(pwd)/config.toml:/config.toml \  
  -p 8080:8080 \
  -e TFSEX_azdo_ACCESSTOKEN=your_access_token_goes_here
  --rm \
  ukhydrographicoffice/azdoexporter
```

The collected metrics are available at `:8080/metrics`

## Configuration

The exporter is configured through a [TOML](https://github.com/toml-lang/toml) configuration file, the configuration file location can passed into the exporter through the `--config` flag.

If a `--config` flag isn't provided, the exporter looks for a `config.toml` in its current location.

For each server the exporter scrapes, a configuration "block" is needed. Each of these blocks must have a unique name:

```toml
[servers]
    [servers.unique_name_1]
    address = "https://dev.azure.com/devorg"

    [servers.unique_name_2]
    address = "https://dev.azure.com/devorg2"
```

The unique name is added as a label to the metrics and is useful for differentiating between metrics from different servers. Be careful of changing this unique name as it will force the metrics to have a different label which can cause issues problems, especially on dashboards

Access tokens for Azure DevOps should be provided using environment variables. The required name of the environment variable is in the format `TFSEX_unique_name_1_ACCESSTOKEN`. Access tokens can be added through the configuration file (see [Full Configuration](#Full-Configuration)) but is discouraged.

The default port and url where the metrics are exposed is `:8080/metrics`

### Basic Configuration

```toml
[servers]
    # On Prem Azure DevOps Server
    [servers.AzDo]
    address = "http://azdo:8080/azdo"
    defaultCollection = "dc"

    # Azure DevOps
    [servers.AzureDevOps]
    address = "https://dev.azure.com/devorg"

    # Azure DevOps
    [servers.OtherAzureDevOpsInstance]
    address = "https://dev.azure.com/devorg2"

# As the access tokens aren't specified, the exporter requires them to be set in environment variables:
# TFSEX_AzDo_ACCESSTOKEN
# TFSEX_AzureDevOps_ACCESSTOKEN
# TFSEX_OtherAzureDevOpsInstance_ACCESSTOKEN
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
    endpoint = "/azdometrics"

[servers]
    [servers.azuredevops]
    address = "https://dev.azure.com/devorg"
    useProxy = true
    accessToken = "thisisamadeupaccesstoken"

    [servers.AzDoInstance]
    address = "http://azdo:8080/azdo"
    defaultCollection = "dc"
    # As the access token isn't specified, an environment variable called TFSEX_TFSInstance_ACCESSTOKEN needs to exist

[proxy]
    url = "http://proxy.devorg.com:9191"
```

## Tips

Set the Prometheus scrape timeout to be larger than 10 seconds as scrapes can sometimes be longer 10s.

## Metrics Exposed

- tfs_build_agents_total
  - Gauge of the total installed build agents. Has labels of `"enabled", "status", "pool" "name"`
- tfs_build_agents_total_scrape_duration_seconds
  - Gauge of duration of time it took to scrape total of installed build agents. Has labels of `"name"`
- tfs_pool_queued_jobs
  - Gauge of the total of queued jobs for pool. Has labels of `"pool"`
  - A queued job is a job that has not yet started. If you have 6 build agents and 7 jobs, 6 jobs will be assigned to the agents, leaving one not started. `tfs_pool_queued_jobs` will then display `1`
- tfs_pool_running_jobs
  - Gauge of the total of running jobs for pool. Has labels of `"pool"`
- tfs_pool_total_jobs
  - Gauge of the total of jobs for pool, this is the sum of running and queued. Has labels of `"pool"`
- tfs_pool_job_total_length_secs
  - Histogram of total length of a job duration in a pool, combining both queued and running. Has labels of `"pool"`
- tfs_pool_job_queue_length_secs
  - Histogram of the length of the time a job spent queued. Has labels of `"pool"`
- tfs_pool_job_running_length_secs
  - Histogram of the length of time a job spent running. Has labels of `"pool"`
