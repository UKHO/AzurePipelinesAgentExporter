# TFS exporter

The name set in the TOML is VERY important (all metrics get tagged with this name so cannot be changed later on)

Set prom timeout to be above 10s as sometimes requests can be above that

Access tokens can be added via EnvVar TFSEX_{ServerName}_ACCESSTOKEN

Add proxy to config (examples)