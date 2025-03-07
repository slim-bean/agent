---
title: grafana-agent convert
weight: 100
labels:
  stage: beta
---

# `grafana-agent convert` command

The `grafana-agent convert` command converts a supported configuration format
to Grafana Agent Flow River format.

## Usage

Usage: `grafana-agent convert [FLAG ...] FILE_NAME`

If the `FILE_NAME` argument is not provided or if the `FILE_NAME` argument is
equal to `-`, `grafana-agent convert` converts the contents of standard input. Otherwise,
`grafana-agent convert` reads and converts the file from disk specified by the argument.

The `--output` flag can be specified to write the contents of the converted
config to the specified path.

The `--bypass-errors` flag can be used to bypass [errors].

The command fails if the source config has syntactically incorrect
configuration or cannot be converted to Grafana Agent Flow River format.

The following flags are supported:

* `--output`, `-o`: The filepath and filename where the output is written.

* `--source-format`, `-f`: Required. The format of the source file. Supported formats: [prometheus].

* `--bypass-errors`, `-b`: Enable bypassing errors when converting.

[prometheus]: #prometheus
[errors]: #errors

### Defaults

Flow Defaults are managed as follows:
* If a provided source config value matches a Flow default value, the
property is left off the Flow output.
* If a non-provided source config value default matches a Flow default value,
the property is left off the Flow output.
* If a non-provided source config value default doesn't match a Flow default
value, the Flow default value is included in the Flow output.

### Errors

Errors are defined as non-critical issues identified during the conversion
where an output can still be generated. These can be bypassed using the
`--bypass-errors` flag.

### Prometheus

Using the `--source-format=prometheus` will convert the source config from
[Prometheus v2.42](https://prometheus.io/docs/prometheus/2.42/configuration/configuration/)
to Grafana Agent Flow config.

This includes Prometheus features such as
[scrape_config](https://prometheus.io/docs/prometheus/2.42/configuration/configuration/#scrape_config), 
[relabel_config](https://prometheus.io/docs/prometheus/2.42/configuration/configuration/#relabel_config),
[metric_relabel_configs](https://prometheus.io/docs/prometheus/2.42/configuration/configuration/#metric_relabel_configs),
[remote_write](https://prometheus.io/docs/prometheus/2.42/configuration/configuration/#remote_write),
and many supported *_sd_configs. Unsupported features in a source config result
in [errors].

