---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-configuration-telemetry"
description: |-
  These options enable and configure telemetry.  
---

# Telemetry Options

These options enable and configure telemetry. 

*   <a name="telemetry"></a><a href="#telemetry">`telemetry`</a> This is a nested object that configures where Consul
    sends its runtime telemetry, and contains the following keys:

    * <a name="telemetry-circonus_api_token"></a><a href="#telemetry-circonus_api_token">`circonus_api_token`</a>
      A valid API Token used to create/manage check. If provided, metric management is enabled.

    * <a name="telemetry-circonus_api_app"></a><a href="#telemetry-circonus_api_app">`circonus_api_app`</a>
      A valid app name associated with the API token. By default, this is set to "consul".

    * <a name="telemetry-circonus_api_url"></a><a href="#telemetry-circonus_api_url">`circonus_api_url`</a>
      The base URL to use for contacting the Circonus API. By default, this is set to "https://api.circonus.com/v2".

    * <a name="telemetry-circonus_submission_interval"></a><a href="#telemetry-circonus_submission_interval">`circonus_submission_interval`</a>
      The interval at which metrics are submitted to Circonus. By default, this is set to "10s" (ten seconds).

    * <a name="telemetry-circonus_submission_url"></a><a href="#telemetry-circonus_submission_url">`circonus_submission_url`</a>
      The `check.config.submission_url` field, of a Check API object, from a previously created HTTPTRAP check.

    * <a name="telemetry-circonus_check_id"></a><a href="#telemetry-circonus_check_id">`circonus_check_id`</a>
      The Check ID (not **check bundle**) from a previously created HTTPTRAP check. The numeric portion of the `check._cid` field in the Check API object.

    * <a name="telemetry-circonus_check_force_metric_activation"></a><a href="#telemetry-circonus_check_force_metric_activation">`circonus_check_force_metric_activation`</a>
      Force activation of metrics which already exist and are not currently active. If check management is enabled, the default behavior is to add new metrics as they are encountered. If the metric already exists in the check, it will **not** be activated. This setting overrides that behavior. By default, this is set to false.

    * <a name="telemetry-circonus_check_instance_id"></a><a href="#telemetry-circonus_check_instance_id">`circonus_check_instance_id`</a>
      Uniquely identifies the metrics coming from this *instance*. It can be used to maintain metric continuity with transient or ephemeral instances as they move around within an infrastructure. By default, this is set to hostname:application name (e.g. "host123:consul").

    * <a name="telemetry-circonus_check_search_tag"></a><a href="#telemetry-circonus_check_search_tag">`circonus_check_search_tag`</a>
      A special tag which, when coupled with the instance id, helps to narrow down the search results when neither a Submission URL or Check ID is provided. By default, this is set to service:application name (e.g. "service:consul").

    * <a name="telemetry-circonus_check_display_name"></a><a href="#telemetry-circonus_check_display_name">`circonus_check_display_name`</a>
      Specifies a name to give a check when it is created. This name is displayed in the Circonus UI Checks list. Available in Consul 0.7.2 and later.

    * <a name="telemetry-circonus_check_tags"></a><a href="#telemetry-circonus_check_tags">`circonus_check_tags`</a>
      Comma separated list of additional tags to add to a check when it is created. Available in Consul 0.7.2 and later.

    * <a name="telemetry-circonus_broker_id"></a><a href="#telemetry-circonus_broker_id">`circonus_broker_id`</a>
      The ID of a specific Circonus Broker to use when creating a new check. The numeric portion of `broker._cid` field in a Broker API object. If metric management is enabled and neither a Submission URL nor Check ID is provided, an attempt will be made to search for an existing check using Instance ID and Search Tag. If one is not found, a new HTTPTRAP check will be created. By default, this is not used and a random Enterprise Broker is selected, or the default Circonus Public Broker.

    * <a name="telemetry-circonus_broker_select_tag"></a><a href="#telemetry-circonus_broker_select_tag">`circonus_broker_select_tag`</a>
      A special tag which will be used to select a Circonus Broker when a Broker ID is not provided. The best use of this is to as a hint for which broker should be used based on *where* this particular instance is running (e.g. a specific geo location or datacenter, dc:sfo). By default, this is left blank and not used.

    * <a name="telemetry-disable_hostname"></a><a href="#telemetry-disable_hostname">`disable_hostname`</a>
      This controls whether or not to prepend runtime telemetry with the machine's hostname, defaults to false.

    * <a name="telemetry-dogstatsd_addr"></a><a href="#telemetry-dogstatsd_addr">`dogstatsd_addr`</a> This provides the
      address of a DogStatsD instance in the format `host:port`. DogStatsD is a protocol-compatible flavor of
      statsd, with the added ability to decorate metrics with tags and event information. If provided, Consul will
      send various telemetry information to that instance for aggregation. This can be used to capture runtime
      information.

    * <a name="telemetry-dogstatsd_tags"></a><a href="#telemetry-dogstatsd_tags">`dogstatsd_tags`</a> This provides a list of global tags
      that will be added to all telemetry packets sent to DogStatsD. It is a list of strings, where each string
      looks like "my_tag_name:my_tag_value".

    * <a name="telemetry-filter_default"></a><a href="#telemetry-filter_default">`filter_default`</a>
     This controls whether to allow metrics that have not been specified by the filter. Defaults to `true`, which will
     allow all metrics when no filters are provided. When set to `false` with no filters, no metrics will be sent.

    * <a name="telemetry-metrics_prefix"></a><a href="#telemetry-metrics_prefix">`metrics_prefix`</a>
      The prefix used while writing all telemetry data. By default, this is set to "consul". This was added
      in Consul 1.0. For previous versions of Consul, use the config option `statsite_prefix` in this
      same structure. This was renamed in Consul 1.0 since this prefix applied to all telemetry providers,
      not just statsite.

    * <a name="telemetry-prefix_filter"></a><a href="#telemetry-prefix_filter">`prefix_filter`</a>
      This is a list of filter rules to apply for allowing/blocking metrics by prefix in the following format:

        ```javascript
        [
          "+consul.raft.apply",
          "-consul.http",
          "+consul.http.GET"
        ]
        ```
      A leading "<b>+</b>" will enable any metrics with the given prefix, and a leading "<b>-</b>" will block them. If there
      is overlap between two rules, the more specific rule will take precedence. Blocking will take priority if the same
      prefix is listed multiple times.

    * <a name="telemetry-prometheus_retention_time"></a><a href="#telemetry-prometheus_retention_time">prometheus_retention_time</a>
      If the value is greater than `0s` (the default), this enables [Prometheus](https://prometheus.io/) export of metrics.
      The duration can be expressed using the duration semantics and will aggregates all counters for the duration specified
      (it might have an impact on Consul's memory usage). A good value for this parameter is at least 2 times the interval of scrape
      of Prometheus, but you might also put a very high retention time such as a few days (for instance 744h to enable retention
      to 31 days).
      Fetching the metrics using prometheus can then be performed using the [`/v1/agent/metrics?format=prometheus`](/api/agent.html#view-metrics) endpoint.
      The format is compatible natively with prometheus. When running in this mode, it is recommended to also enable the option
      <a href="#telemetry-disable_hostname">`disable_hostname`</a> to avoid having prefixed metrics with hostname.
      Consul does not use the default Prometheus path, so Prometheus must be configured as follows.
      Note that using ?format=prometheus in the path won't work as ? will be escaped, so it must be specified as a parameter.

        ```yaml
          metrics_path: "/v1/agent/metrics"
          params:
            format: ['prometheus']
        ```

    * <a name="telemetry-statsd_address"></a><a href="#telemetry-statsd_address">`statsd_address`</a> This provides the
      address of a statsd instance in the format `host:port`. If provided, Consul will send various telemetry information to that instance for
      aggregation. This can be used to capture runtime information. This sends UDP packets only and can be used with
      statsd or statsite.

    * <a name="telemetry-statsite_address"></a><a href="#telemetry-statsite_address">`statsite_address`</a> This provides
      the address of a statsite instance in the format `host:port`. If provided, Consul will stream various telemetry information to that instance
      for aggregation. This can be used to capture runtime information. This streams via TCP and can only be used with
      statsite.