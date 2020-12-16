/*eslint no-console: "off"*/
(function () {
  var emptySeries = { unitSuffix: '', labels: {}, data: [] };

  var prometheusProvider = {
    options: {},

    /**
     * init is called when the provider is first loaded.
     *
     * options.providerOptions contains any operator configured parameters
     * specified in the Consul agent config that is serving the UI.
     *
     * Consul will provide:
     *
     * 1. A boolean options.metrics_proxy_enabled to indicate whether the agent
     * has a metrics proxy configured.
     * 2. A fetch-like options.fetch which is a thin fetch wrapper that prefixes
     * any url with the url of Consul's proxy endpoint and adds your current
     * Consul ACL token to the request headers. Otherwise it functions like the
     * browsers native fetch
     *
     * The provider should throw an Exception if the options are not valid for
     * example because it requires a metrics proxy and one is not configured.
     */
    init: function (options) {
      this.options = options;
      if (!this.options.metrics_proxy_enabled) {
        throw new Error(
          'prometheus metrics provider currently requires the ui_config.metrics_proxy to be configured in the Consul agent.'
        );
      }
    },

    // simple httpGet function that also encodes query parameters
    // before passing the constructed url through to native fetch
    // any errors should throw an error with a statusCode property
    httpGet: function (url, queryParams, headers) {
      if (queryParams) {
        var separator = url.indexOf('?') !== -1 ? '&' : '?';
        var qs = Object.keys(queryParams)
          .map(function (key) {
            return encodeURIComponent(key) + '=' + encodeURIComponent(queryParams[key]);
          })
          .join('&');
        url = url + separator + qs;
      }
      // fetch the url along with any headers
      return this.options.fetch(url, { headers: headers || {} }).then(function (response) {
        if (response.ok) {
          return response.json();
        } else {
          // throw a statusCode error if any errors are received
          var e = new Error('HTTP Error: ' + response.statusText);
          e.statusCode = response.status;
          throw e;
        }
      });
    },

    /**
     * serviceRecentSummarySeries should return time series for a recent time
     * period summarizing the usage of the named service in the indicated
     * datacenter. In Consul Enterprise a non-empty namespace is also provided.
     *
     * If these metrics aren't available then an empty series array may be
     * returned.
     *
     * The period may (later) be specified in options.startTime and
     * options.endTime.
     *
     * The service's protocol must be given as one of Consul's supported
     * protocols e.g. "tcp", "http", "http2", "grpc". If it is empty or the
     * provider doesn't recognize the protocol, it should treat it as "tcp" and
     * provide basic connection stats.
     *
     * The expected return value is a promise which resolves to an object that
     * should look like the following:
     *
     *  {
     *    // The unitSuffix is shown after the value in tooltips. Values will be
     *    // rounded and shortened. Larger values will already have a suffix
     *    // like "10k". The suffix provided here is concatenated directly
     *    // allowing for suffixes like "mbps/kbps" by using a suffix of "bps".
     *    // If the unit doesn't make sense in this format, include a
     *    // leading space for example " rps" would show as "1.2k rps".
     *    unitSuffix: " rps",
     *
     *    // The set of labels to graph. The key should exactly correspond to a
     *    // property of every data point in the array below except for the
     *    // special case "Total" which is used to show the sum of all the
     *    // stacked graph values. The key is displayed in the tooltop so it
     *    // should be human-friendly but as concise as possible. The value is a
     *    // longer description that is displayed in the graph's key on request
     *    // to explain exactly what the metrics mean.
     *    labels: {
     *      "Total": "Total inbound requests per second.",
     *      "Successes": "Successful responses (with an HTTP response code ...",
     *      "Errors": "Error responses (with an HTTP response code in the ...",
     *    },
     *
     *    data: [
     *      {
     *        time: 1600944516286, // milliseconds since Unix epoch
     *        "Successes": 1234.5,
     *        "Errors": 2.3,
     *      },
     *      ...
     *    ]
     *  }
     *
     *  Every data point object should have a value for every series label
     *  (except for "Total") otherwise it will be assumed to be "0".
     */
    serviceRecentSummarySeries: function (service, dc, nspace, protocol, options) {
      // Fetch time-series
      var series = [];
      var labels = [];

      // Set the start and end range here so that all queries end up with
      // identical time axes. Later we might accept these as options.
      var now = new Date().getTime() / 1000;
      options.start = now - 15 * 60;
      options.end = now;

      if (this.hasL7Metrics(protocol)) {
        return this.fetchRequestRateSeries(service, dc, nspace, options);
      }

      // Fallback to just L4 metrics.
      return this.fetchDataRateSeries(service, dc, nspace, options);
    },

    /**
     * serviceRecentSummaryStats should return four summary statistics for a
     * recent time period for the named service in the indicated datacenter. In
     * Consul Enterprise a non-empty namespace is also provided.
     *
     * If these metrics aren't available then an empty array may be returned.
     *
     * The period may (later) be specified in options.startTime and
     * options.endTime.
     *
     * The service's protocol must be given as one of Consul's supported
     * protocols e.g. "tcp", "http", "http2", "grpc". If it is empty or the
     * provider doesn't recognize it it should treat it as "tcp" and provide
     * just basic connection stats.
     *
     * The expected return value is a promise which resolves to an object that
     * should look like the following:
     *
     *  {
     *    stats: [ // We expect four of these for now.
     *      {
     *        // label should be 3 chars or fewer as an abbreviation
     *        label: "SR",
     *
     *        // desc describes the stat in a tooltip
     *        desc: "Success Rate - the percentage of all requests that were not 5xx status",
     *
     *        // value is a string allowing the provider to format it and add
     *        // units as appropriate. It should be as compact as possible.
     *        value: "98%",
     *      }
     *    ]
     *  }
     */
    serviceRecentSummaryStats: function (service, dc, nspace, protocol, options) {
      // Fetch stats
      var stats = [];
      if (this.hasL7Metrics(protocol)) {
        stats.push(this.fetchRPS(service, dc, nspace, 'service', options));
        stats.push(this.fetchER(service, dc, nspace, 'service', options));
        stats.push(this.fetchPercentile(50, service, dc, nspace, 'service', options));
        stats.push(this.fetchPercentile(99, service, dc, nspace, 'service', options));
      } else {
        // Fallback to just L4 metrics.
        stats.push(this.fetchConnRate(service, dc, nspace, 'service', options));
        stats.push(this.fetchServiceRx(service, dc, nspace, 'service', options));
        stats.push(this.fetchServiceTx(service, dc, nspace, 'service', options));
        stats.push(this.fetchServiceNoRoute(service, dc, nspace, 'service', options));
      }
      return this.fetchStats(stats);
    },

    /**
     * upstreamRecentSummaryStats should return four summary statistics for each
     * upstream service over a recent time period, relative to the named service
     * in the indicated datacenter. In Consul Enterprise a non-empty namespace
     * is also provided.
     *
     * Note that the upstreams themselves might be in different datacenters but
     * we only pass the target service DC since typically these metrics should
     * be from the outbound listener of the target service in this DC even if
     * they eventually end up in another DC.
     *
     * If these metrics aren't available then an empty array may be returned.
     *
     * The period may (later) be specified in options.startTime and
     * options.endTime.
     *
     * The expected return value format is shown below:
     *
     *   {
     *     stats: {
     *       // Each upstream will appear as an entry keyed by the upstream
     *       // service name. The value is an array of stats with the same
     *       // format as serviceRecentSummaryStats response.stats. Note that
     *       // different upstreams might show different stats depending on
     *       // their protocol.
     *       "upstream_name": [
     *         {label: "SR", desc: "...", value: "99%"},
     *         ...
     *       ],
     *       ...
     *     }
     *   }
     */
    upstreamRecentSummaryStats: function (service, dc, nspace, options) {
      return this.fetchRecentSummaryStats(service, dc, nspace, 'upstream', options);
    },

    /**
     * downstreamRecentSummaryStats should return four summary statistics for
     * each downstream service over a recent time period, relative to the named
     * service in the indicated datacenter. In Consul Enterprise a non-empty
     * namespace is also provided.
     *
     * Note that the service may have downstreams in different datacenters. For
     * some metrics systems which are per-datacenter this makes it hard to query
     * for all downstream metrics from one source. For now the UI will only show
     * downstreams in the same datacenter as the target service. In the future
     * this method may be called multiple times, once for each DC that contains
     * downstream services to gather metrics from each. In that case a separate
     * option for target datacenter will be used since the target service's DC
     * is still needed to correctly identify the outbound clusters that will
     * route to it from the remote DC.
     *
     * If these metrics aren't available then an empty array may be returned.
     *
     * The period may (later) be specified in options.startTime and
     * options.endTime.
     *
     * The expected return value format is shown below:
     *
     *   {
     *     stats: {
     *       // Each downstream will appear as an entry keyed by "service.namespace.dc".
     *       // The value is an array of stats with the same
     *       // format as serviceRecentSummaryStats response.stats. Different
     *       // downstreams may display different stats if required although the
     *       // protocol should be the same for all as it is the target
     *       // service's protocol that matters here.
     *       "web.default.dc1": [
     *         {label: "SR", desc: "...", value: "99%"},
     *         ...
     *       ],
     *       ...
     *     }
     *   }
     */
    downstreamRecentSummaryStats: function (service, dc, nspace, options) {
      return this.fetchRecentSummaryStats(service, dc, nspace, 'downstream', options);
    },

    fetchRecentSummaryStats: function (service, dc, nspace, type, options) {
      // Fetch stats
      var stats = [];

      // We don't know which upstreams are HTTP/TCP so just fetch all of them.

      // HTTP
      stats.push(this.fetchRPS(service, dc, nspace, type, options));
      stats.push(this.fetchER(service, dc, nspace, type, options));
      stats.push(this.fetchPercentile(50, service, dc, nspace, type, options));
      stats.push(this.fetchPercentile(99, service, dc, nspace, type, options));

      // L4
      stats.push(this.fetchConnRate(service, dc, nspace, type, options));
      stats.push(this.fetchServiceRx(service, dc, nspace, type, options));
      stats.push(this.fetchServiceTx(service, dc, nspace, type, options));
      stats.push(this.fetchServiceNoRoute(service, dc, nspace, type, options));

      return this.fetchStatsGrouped(stats);
    },

    hasL7Metrics: function (protocol) {
      return protocol === 'http' || protocol === 'http2' || protocol === 'grpc';
    },

    fetchStats: function (statsPromises) {
      var all = Promise.all(statsPromises).then(function (results) {
        var data = {
          stats: [],
        };
        // Add all non-empty stats
        for (var i = 0; i < statsPromises.length; i++) {
          if (results[i].value) {
            data.stats.push(results[i]);
          }
        }
        return data;
      });

      // Fetch the metrics async, and return a promise to the result.
      return all;
    },

    fetchStatsGrouped: function (statsPromises) {
      var all = Promise.all(statsPromises).then(function (results) {
        var data = {
          stats: {},
        };
        // Add all non-empty stats
        for (var i = 0; i < statsPromises.length; i++) {
          if (results[i]) {
            for (var group in results[i]) {
              if (!results[i].hasOwnProperty(group)) continue;
              if (!data.stats[group]) {
                data.stats[group] = [];
              }
              data.stats[group].push(results[i][group]);
            }
          }
        }
        return data;
      });

      // Fetch the metrics async, and return a promise to the result.
      return all;
    },

    reformatSeries: function (unitSuffix, labelMap) {
      return function (response) {
        // Handle empty result sets gracefully.
        if (
          !response.data ||
          !response.data.result ||
          response.data.result.length == 0 ||
          !response.data.result[0].values ||
          response.data.result[0].values.length == 0
        ) {
          return emptySeries;
        }
        // Reformat the prometheus data to be the format we want with stacked
        // values as object properties.

        // Populate time values first based on first result since Prometheus will
        // always return all the same points for all series in the query.
        let series = response.data.result[0].values.map(function (d, i) {
          return {
            time: Math.round(d[0] * 1000),
          };
        });

        // Then for each series returned populate the labels and values in the
        // points.
        response.data.result.map(function (d) {
          d.values.map(function (p, i) {
            series[i][d.metric.label] = parseFloat(p[1]);
          });
        });

        return {
          unitSuffix: unitSuffix,
          labels: labelMap,
          data: series,
        };
      };
    },

    fetchRequestRateSeries: function (service, dc, nspace, options) {
      // We need the sum of all non-500 error rates as one value and the 500
      // error rate as a separate series so that they stack to show the full
      // request rate. Some creative label replacement makes this possible in
      // one query.
      var q =
        `sum by (label) (` +
        // The outer label_replace catches 5xx error and relabels them as
        // err=yes
        `label_replace(` +
        // The inner label_replace relabels all !5xx rates as err=no so they
        // will get summed together.
        `label_replace(` +
        // Get rate of requests to the service
        `irate(envoy_listener_http_downstream_rq_xx{` +
        `consul_source_service="${service}",` +
        `consul_source_datacenter="${dc}",` +
        `consul_source_namespace="${nspace}",` +
        `envoy_http_conn_manager_prefix="public_listener"}[10m])` +
        // ... inner replacement matches all code classes except "5" and
        // applies err=no
        `, "label", "Successes", "envoy_response_code_class", "[^5]")` +
        // ... outer replacement matches code=5 and applies err=yes
        `, "label", "Errors", "envoy_response_code_class", "5")` +
        `)`;
      var labelMap = {
        Total: 'Total inbound requests per second',
        Successes:
          'Successful responses (with an HTTP response code not in the 5xx range) per second.',
        Errors: 'Error responses (with an HTTP response code in the 5xx range) per second.',
      };
      return this.fetchSeries(q, options).then(this.reformatSeries(' rps', labelMap));
    },

    fetchDataRateSeries: function (service, dc, nspace, options) {
      // 8 * converts from bytes/second to bits/second
      var q =
        `8 * sum by (label) (` +
        // Label replace generates a unique label per rx/tx metric to stop them
        // being summed together.
        `label_replace(` +
        // Get the tx rate
        `irate(envoy_tcp_downstream_cx_tx_bytes_total{` +
        `consul_source_service="${service}",` +
        `consul_source_datacenter="${dc}",` +
        `consul_source_namespace="${nspace}",` +
        `envoy_tcp_prefix="public_listener"}[10m])` +
        // Match all and apply the tx label
        `, "label", "Outbound", "__name__", ".*"` +
        // Union those vectors with the RX ones
        `) or label_replace(` +
        // Get the rx rate
        `irate(envoy_tcp_downstream_cx_rx_bytes_total{` +
        `consul_source_service="${service}",` +
        `consul_source_datacenter="${dc}",` +
        `consul_source_namespace="${nspace}",` +
        `envoy_tcp_prefix="public_listener"}[10m])` +
        // Match all and apply the rx label
        `, "label", "Inbound", "__name__", ".*"` +
        `)` +
        `)`;
      var labelMap = {
        Total: 'Total bandwidth',
        Inbound: 'Inbound data rate (data recieved) from the network in bits per second.',
        Outbound: 'Outbound data rate (data transmitted) from the network in bits per second.',
      };
      return this.fetchSeries(q, options).then(this.reformatSeries('bps', labelMap));
    },

    makeSubject: function (service, dc, nspace, type) {
      var entity = `${nspace}/${service} (${dc})`;
      if (type == 'upstream') {
        // {{GROUP}} is a placeholder that is replaced by the upstream name
        return `${entity} &rarr; {{GROUP}}`;
      }
      if (type == 'downstream') {
        // {{GROUP}} is a placeholder that is replaced by the downstream name
        return `{{GROUP}} &rarr; ${entity}`;
      }
      return entity;
    },

    makeHTTPSelector: function (service, dc, nspace, type) {
      // Downstreams are totally different
      if (type == 'downstream') {
        return `consul_destination_service="${service}",consul_destination_datacenter="${dc}",consul_destination_namespace="${nspace}"`;
      }
      var lc = `consul_source_service="${service}",consul_source_datacenter="${dc}",consul_source_namespace="${nspace}"`;
      if (type == 'upstream') {
        lc += `,envoy_http_conn_manager_prefix="upstream"`;
      } else {
        // Only care about inbound public listener
        lc += `,envoy_http_conn_manager_prefix="public_listener"`;
      }
      return lc;
    },

    makeTCPSelector: function (service, dc, nspace, type) {
      // Downstreams are totally different
      if (type == 'downstream') {
        return `consul_destination_service="${service}",consul_destination_datacenter="${dc}",consul_destination_namespace="${nspace}"`;
      }
      var lc = `consul_source_service="${service}",consul_source_datacenter="${dc}",consul_source_namespace="${nspace}"`;
      if (type == 'upstream') {
        lc += `,envoy_tcp_prefix=~"upstream.*"`;
      } else {
        // Only care about inbound public listener
        lc += `,envoy_tcp_prefix="public_listener"`;
      }
      return lc;
    },

    groupQuery: function (type, q) {
      if (type == 'upstream') {
        q += ' by (consul_upstream_service,consul_upstream_datacenter,consul_upstream_namespace)';
      } else if (type == 'downstream') {
        q += ' by (consul_source_service,consul_source_datacenter,consul_source_namespace)';
      }
      return q;
    },

    groupByInfix: function (type) {
      if (type == 'upstream') {
        return 'upstream';
      } else if (type == 'downstream') {
        return 'source';
      } else {
        return false;
      }
    },

    metricPrefixHTTP: function (type) {
      if (type == 'downstream') {
        return 'envoy_cluster_upstream_rq';
      }
      return 'envoy_http_downstream_rq';
    },

    metricPrefixTCP: function (type) {
      if (type == 'downstream') {
        return 'envoy_cluster_upstream_cx';
      }
      return 'envoy_tcp_downstream_cx';
    },

    fetchRPS: function (service, dc, nspace, type, options) {
      var sel = this.makeHTTPSelector(service, dc, nspace, type);
      var subject = this.makeSubject(service, dc, nspace, type);
      var metricPfx = this.metricPrefixHTTP(type);
      var q = `sum(rate(${metricPfx}_completed{${sel}}[15m]))`;
      return this.fetchStat(
        this.groupQuery(type, q),
        'RPS',
        `<b>${subject}</b> request rate averaged over the last 15 minutes`,
        shortNumStr,
        this.groupByInfix(type)
      );
    },

    fetchER: function (service, dc, nspace, type, options) {
      var sel = this.makeHTTPSelector(service, dc, nspace, type);
      var subject = this.makeSubject(service, dc, nspace, type);
      var groupBy = '';
      if (type == 'upstream') {
        groupBy +=
          ' by (consul_upstream_service,consul_upstream_datacenter,consul_upstream_namespace)';
      } else if (type == 'downstream') {
        groupBy += ' by (consul_source_service,consul_source_datacenter,consul_source_namespace)';
      }
      var metricPfx = this.metricPrefixHTTP(type);
      var q = `sum(rate(${metricPfx}_xx{${sel},envoy_response_code_class="5"}[15m]))${groupBy}/sum(rate(${metricPfx}_xx{${sel}}[15m]))${groupBy}`;
      return this.fetchStat(
        q,
        'ER',
        `Percentage of <b>${subject}</b> requests which were 5xx status over the last 15 minutes`,
        function (val) {
          return shortNumStr(val) + '%';
        },
        this.groupByInfix(type)
      );
    },

    fetchPercentile: function (percentile, service, dc, nspace, type, options) {
      var sel = this.makeHTTPSelector(service, dc, nspace, type);
      var subject = this.makeSubject(service, dc, nspace, type);
      var groupBy = 'le';
      if (type == 'upstream') {
        groupBy += ',consul_upstream_service,consul_upstream_datacenter,consul_upstream_namespace';
      } else if (type == 'downstream') {
        groupBy += ',consul_source_service,consul_source_datacenter,consul_source_namespace';
      }
      var metricPfx = this.metricPrefixHTTP(type);
      var q = `histogram_quantile(${percentile /
        100}, sum by(${groupBy}) (rate(${metricPfx}_time_bucket{${sel}}[15m])))`;
      return this.fetchStat(
        q,
        `P${percentile}`,
        `<b>${subject}</b> ${percentile}th percentile request service time over the last 15 minutes`,
        shortTimeStr,
        this.groupByInfix(type)
      );
    },

    fetchConnRate: function (service, dc, nspace, type, options) {
      var sel = this.makeTCPSelector(service, dc, nspace, type);
      var subject = this.makeSubject(service, dc, nspace, type);
      var metricPfx = this.metricPrefixTCP(type);
      var q = `sum(rate(${metricPfx}_total{${sel}}[15m]))`;
      return this.fetchStat(
        this.groupQuery(type, q),
        'CR',
        `<b>${subject}</b> inbound TCP connections per second averaged over the last 15 minutes`,
        shortNumStr,
        this.groupByInfix(type)
      );
    },

    fetchServiceRx: function (service, dc, nspace, type, options) {
      var sel = this.makeTCPSelector(service, dc, nspace, type);
      var subject = this.makeSubject(service, dc, nspace, type);
      var metricPfx = this.metricPrefixTCP(type);
      var q = `8 * sum(rate(${metricPfx}_rx_bytes_total{${sel}}[15m]))`;
      return this.fetchStat(
        this.groupQuery(type, q),
        'RX',
        `<b>${subject}</b> received bits per second averaged over the last 15 minutes`,
        shortNumStr,
        this.groupByInfix(type)
      );
    },

    fetchServiceTx: function (service, dc, nspace, type, options) {
      var sel = this.makeTCPSelector(service, dc, nspace, type);
      var subject = this.makeSubject(service, dc, nspace, type);
      var metricPfx = this.metricPrefixTCP(type);
      var q = `8 * sum(rate(${metricPfx}_tx_bytes_total{${sel}}[15m]))`;
      var self = this;
      return this.fetchStat(
        this.groupQuery(type, q),
        'TX',
        `<b>${subject}</b> transmitted bits per second averaged over the last 15 minutes`,
        shortNumStr,
        this.groupByInfix(type)
      );
    },

    fetchServiceNoRoute: function (service, dc, nspace, type, options) {
      var sel = this.makeTCPSelector(service, dc, nspace, type);
      var subject = this.makeSubject(service, dc, nspace, type);
      var metricPfx = this.metricPrefixTCP(type);
      var metric = '_no_route';
      if (type == 'downstream') {
        metric = '_connect_fail';
      }
      var q = `sum(rate(${metricPfx}${metric}{${sel}}[15m]))`;
      return this.fetchStat(
        this.groupQuery(type, q),
        'NR',
        `<b>${subject}</b> unroutable (failed) connections per second averaged over the last 15 minutes`,
        shortNumStr,
        this.groupByInfix(type)
      );
    },

    fetchStat: function (promql, label, desc, formatter, groupByInfix) {
      if (!groupByInfix) {
        // If we don't have a grouped result and its just a single stat, return
        // no result as a zero not a missing stat.
        promql += ' OR on() vector(0)';
      }
      var params = {
        query: promql,
        time: new Date().getTime() / 1000,
      };
      return this.httpGet('/api/v1/query', params).then(function (response) {
        if (!groupByInfix) {
          // Not grouped, expect just one stat value return that
          var v = parseFloat(response.data.result[0].value[1]);
          return {
            label: label,
            desc: desc,
            value: isNaN(v) ? '-' : formatter(v),
          };
        }

        var data = {};
        for (var i = 0; i < response.data.result.length; i++) {
          var res = response.data.result[i];
          var v = parseFloat(res.value[1]);
          var service = res.metric['consul_' + groupByInfix + '_service'];
          var nspace = res.metric['consul_' + groupByInfix + '_namespace'];
          var datacenter = res.metric['consul_' + groupByInfix + '_datacenter'];
          var groupName = `${service}.${nspace}.${datacenter}`;
          data[groupName] = {
            label: label,
            desc: desc.replace('{{GROUP}}', groupName),
            value: isNaN(v) ? '-' : formatter(v),
          };
        }
        return data;
      });
    },

    fetchSeries: function (promql, options) {
      var params = {
        query: promql,
        start: options.start,
        end: options.end,
        step: '10s',
        timeout: '8s',
      };
      return this.httpGet('/api/v1/query_range', params);
    },
  };

  // Helper functions
  function shortNumStr(n) {
    if (n < 1e3) {
      if (Number.isInteger(n)) return '' + n;
      if (n >= 100) {
        // Go to 3 significant figures but wrap it in Number to avoid scientific
        // notation lie 2.3e+2 for 230.
        return Number(n.toPrecision(3));
      }
      if (n < 1) {
        // Very small numbers show with limited precision to prevent long string
        // of 0.000000.
        return Number(n.toFixed(2));
      } else {
        // Two sig figs is enough below this
        return Number(n.toPrecision(2));
      }
    }
    if (n >= 1e3 && n < 1e6) return +(n / 1e3).toPrecision(3) + 'k';
    if (n >= 1e6 && n < 1e9) return +(n / 1e6).toPrecision(3) + 'm';
    if (n >= 1e9 && n < 1e12) return +(n / 1e9).toPrecision(3) + 'g';
    if (n >= 1e12) return +(n / 1e12).toFixed(0) + 't';
  }

  function shortTimeStr(n) {
    if (n < 1e3) return Math.round(n) + 'ms';

    var secs = n / 1e3;
    if (secs < 60) return secs.toFixed(1) + 's';

    var mins = secs / 60;
    if (mins < 60) return mins.toFixed(1) + 'm';

    var hours = mins / 60;
    if (hours < 24) return hours.toFixed(1) + 'h';

    var days = hours / 24;
    return days.toFixed(1) + 'd';
  }

  /* global consul:writable */
  window.consul.registerMetricsProvider('prometheus', prometheusProvider);
})();
