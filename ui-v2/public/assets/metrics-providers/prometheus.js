(function () {
  var prometheusProvider = {
    options: {},

    /**
     * init is called when the provide is first loaded.
     *
     * options.providerOptions contains any operator configured parameters
     * specified in the Consul agent config that is serving the UI.
     *
     * options.proxy.baseURL contains the base URL if the agent has a metrics
     * proxy configured. If it doesn't options.proxy will be null. The provider
     * should throw an Exception (TODO: specific type?) if it requires a metrics
     * proxy and one is not configured.
     */
    init: function(options) {
      this.options = options;
    },

    /**
     * serviceRecentSummarySeries should return time series for a recent time
     * period summarizing the usage of the named service.
     *
     * If these metrics aren't available then empty series may be returned.
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
     *    series: [
     *      {
     *        label: "Requests per second",
     *        data: [...]
     *      },
     *      ...
     *    ]
     *  }
     *
     * Each time series' data array is simple an array of tuples with the first
     * being a Date object and the second a floating point value:
     *
     *   [[Date(1600944516286), 1234.9], [Date(1600944526286), 1234.9], ...]
     */
    serviceRecentSummarySeries: function(serviceName, protocol, options) {
      // Fetch time-series
      var series = []
      var labels = []

      if (this.hasL7Metrics(protocol)) {
        series.push(this.fetchRequestRateSeries(serviceName, options))
        labels.push("Requests per second")
        series.push(this.fetchErrorRateSeries(serviceName, options))
        labels.push("Error rate")
      } else {
        // Fallback to just L4 metrics.
        series.push(this.fetchServiceRxSeries(serviceName, options))
        labels.push("Data rate received")
        series.push(this.fetchServiceTxSeries(serviceName, options))
        labels.push("Data rate transmitted")
      }

      var all = Promise.allSettled(series).
        then(function(results){
        var data = { series: [] }
        for (var i = 0; i < series.length; i++) {
          if (results[i].value) {
            data.series.push({
              label: labels[i],
              data: results[i].value
            });
          } else if (results[i].reason) {
            console.log("ERROR processing series", labels[i], results[i].reason)
          }
        }
        return data
      })

      // Fetch the metrics async, and return a promise to the result.
      return all
    },

    /**
     * serviceRecentSummaryStats should return four summary statistics for a
     * recent time period for the named service.
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
     *        // desc describes the stat in a tooltip
     *        desc: "Success Rate - the percentage of all requests that were not 5xx status",
     *        // value is a string allowing the provider to format it and add
     *        // units as appropriate. It should be as compact as possible.
     *        value: "98%",
     *      }
     *    ]
     *  }
     */
    serviceRecentSummaryStats: function(serviceName, protocol, options) {
      // Fetch stats
      var stats = [];
      if (this.hasL7Metrics(protocol)) {
        stats.push(this.fetchRPS(serviceName, "service", options))
        stats.push(this.fetchER(serviceName, "service", options))
        stats.push(this.fetchPercentile(50, serviceName, "service", options))
        stats.push(this.fetchPercentile(99, serviceName, "service", options))
      } else {
        // Fallback to just L4 metrics.
        stats.push(this.fetchConnRate(serviceName, "service", options))
        stats.push(this.fetchServiceRx(serviceName, "service", options))
        stats.push(this.fetchServiceTx(serviceName, "service", options))
        stats.push(this.fetchServiceNoRoute(serviceName, "service", options))
      }
      return this.fetchStats(stats)
    },

    /**
     * upstreamRecentSummaryStats should return four summary statistics for each
     * upstream service over a recent time period.
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
    upstreamRecentSummaryStats: function(serviceName, upstreamName, options) {
      return this.fetchRecentSummaryStats(serviceName, "upstream", options)
    },

    /**
     * downstreamRecentSummaryStats should return four summary statistics for each
     * downstream service over a recent time period.
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
     *       // Each downstream will appear as an entry keyed by the downstream
     *       // service name. The value is an array of stats with the same
     *       // format as serviceRecentSummaryStats response.stats. Note that
     *       // different downstreams might show different stats depending on
     *       // their protocol.
     *       "downstream_name": [
     *         {label: "SR", desc: "...", value: "99%"},
     *         ...
     *       ],
     *       ...
     *     }
     *   }
     */
    downstreamRecentSummaryStats: function(serviceName, options) {
      return this.fetchRecentSummaryStats(serviceName, "downstream", options)
    },

    fetchRecentSummaryStats: function(serviceName, type, options) {
      // Fetch stats
      var stats = [];

      // We don't know which upstreams are HTTP/TCP so just fetch all of them.

      // HTTP
      stats.push(this.fetchRPS(serviceName, type, options))
      stats.push(this.fetchER(serviceName, type, options))
      stats.push(this.fetchPercentile(50, serviceName, type, options))
      stats.push(this.fetchPercentile(99, serviceName, type, options))

      // L4
      stats.push(this.fetchConnRate(serviceName, type, options))
      stats.push(this.fetchServiceRx(serviceName, type, options))
      stats.push(this.fetchServiceTx(serviceName, type, options))
      stats.push(this.fetchServiceNoRoute(serviceName, type, options))

      return this.fetchStatsGrouped(stats)
    },

    hasL7Metrics: function(protocol) {
      return protocol === "http" || protocol === "http2" || protocol === "grpc"
    },

    fetchStats: function(statsPromises) {
      var all = Promise.allSettled(statsPromises).
        then(function(results){
        var data = {
          stats: []
        }
        // Add all non-empty stats
        for (var i = 0; i < statsPromises.length; i++) {
          if (results[i].value) {
            data.stats.push(results[i].value);
          } else if (results[i].reason) {
            console.log("ERROR processing stat", results[i].reason)
          }
        }
        return data
      })

      // Fetch the metrics async, and return a promise to the result.
      return all
    },

    fetchStatsGrouped: function(statsPromises) {
      var all = Promise.allSettled(statsPromises).
        then(function(results){
        var data = {
          stats: {}
        }
        // Add all non-empty stats
        for (var i = 0; i < statsPromises.length; i++) {
          if (results[i].value) {
            for (var group in results[i].value) {
              if (!results[i].value.hasOwnProperty(group)) continue;
              if (!data.stats[group]) {
                data.stats[group] = []
              }
              data.stats[group].push(results[i].value[group])
            }
          } else if (results[i].reason) {
            console.log("ERROR processing stat", results[i].reason)
          }
        }
        return data
      })

      // Fetch the metrics async, and return a promise to the result.
      return all
    },

    reformatSeries: function(response) {
      // Reformat the prometheus data to be the format we want which is
      // essentially the same but with Date objects instead of unix timestamps.
      return response.data.result[0].values.map(function(val){
        return [new Date(val[0]*1000), parseFloat(val[1])]
      })
    },

    fetchRequestRateSeries: function(serviceName, options){
      var q = `sum(irate(envoy_listener_http_downstream_rq_xx{local_cluster="${serviceName}",envoy_http_conn_manager_prefix="public_listener_http"}[10m]))`
      return this.fetchSeries(q, options).then(this.reformatSeries, function(xhr){
        // Failure. log to console and return an blank result for now.
        console.log("ERROR: failed to fetch requestRate", xhr.responseText)
        return []
      })
    },

    fetchErrorRateSeries: function(serviceName, options){
      // 100 * to get a result in percent
      var q = `100 * (`+
        // Sum the rates for all inbound requests to the service's public
        // listener that were 5xx (errors).
        `sum(`+
          `irate(envoy_listener_http_downstream_rq_xx{`+
            `local_cluster="${serviceName}",`+
            `envoy_http_conn_manager_prefix="public_listener_http",`+
            `envoy_response_code_class="5"}[10m]`+
          `)`+
        `)`+
      // Divided by the total request rate (sum over all response code classes)
      `/`+
      `sum(`+
        `irate(envoy_listener_http_downstream_rq_xx{`+
          `local_cluster="${serviceName}",`+
          `envoy_http_conn_manager_prefix="public_listener_http"}[10m]`+
        `)`+
      `)`
      return this.fetchSeries(q, options).then(this.reformatSeries, function(xhr){
        // Failure. log to console and return an blank result for now.
        console.log("ERROR: failed to fetch errorRate", xhr.responseText)
        return []
      })
    },

    fetchServiceRxSeries: function(serviceName, options){
      var q = `sum(irate(envoy_tcp_downstream_cx_rx_bytes_total{local_cluster="${serviceName}", envoy_tcp_prefix="public_listener_tcp"}[10m]))`
      return this.fetchSeries(q, options).then(this.reformatSeries, function(xhr){
        // Failure. log to console and return an blank result for now.
        console.log("ERROR: failed to fetch rx data rate", xhr.responseText)
        return []
      })
    },

    fetchServiceTxSeries: function(serviceName, options){
      var q = `sum(irate(envoy_tcp_downstream_cx_tx_bytes_total{local_cluster="${serviceName}", envoy_tcp_prefix="public_listener_tcp"}[10m]))`
      return this.fetchSeries(q, options).then(this.reformatSeries, function(xhr){
        // Failure. log to console and return an blank result for now.
        console.log("ERROR: failed to fetch tx data rate", xhr.responseText)
        return []
      })
    },

    makeSubject: function(serviceName, type) {
      if (type == "upstream") {
        // {{GROUP}} is a placeholder that is replaced by the upstream name
        return `${serviceName} &rarr; {{GROUP}}`;
      }
      if (type == "downstream") {
        // {{GROUP}} is a placeholder that is replaced by the downstream name
        return `{{GROUP}} &rarr; ${serviceName}`;
      }
      return serviceName
    },

    makeHTTPSelector: function(serviceName, type) {
      // Downstreams are totally different
      if (type == "downstream") {
        return `consul_service="${serviceName}"`
      }
      var lc = `local_cluster="${serviceName}"`
      if (type == "upstream") {
        lc += `,envoy_http_conn_manager_prefix=~"upstream_.*"`;
      } else {
        // Only care about inbound public listener
        lc += `,envoy_http_conn_manager_prefix="public_listener_http"`
      }
      return lc
    },

    makeTCPSelector: function(serviceName, type) {
      // Downstreams are totally different
      if (type == "downstream") {
        return `consul_service="${serviceName}"`
      }
      var lc = `local_cluster="${serviceName}"`
      if (type == "upstream") {
        lc += `,envoy_tcp_prefix=~"upstream_.*"`;
      } else {
        // Only care about inbound public listener
        lc += `,envoy_tcp_prefix="public_listener_tcp"`
      }
      return lc
    },

    groupQueryHTTP: function(type, q) {
      if (type == "upstream") {
        q += " by (envoy_http_conn_manager_prefix)"
        // Extract the raw upstream service name to group results by
        q = this.upstreamRelabelQueryHTTP(q)
      } else if (type == "downstream") {
        q += " by (local_cluster)"
        q = this.downstreamRelabelQuery(q)
      }
      return q
    },

    groupQueryTCP: function(type, q) {
      if (type == "upstream") {
        q += " by (envoy_tcp_prefix)"
        // Extract the raw upstream service name to group results by
        q = this.upstreamRelabelQueryTCP(q)
      } else if (type == "downstream") {
        q += " by (local_cluster)"
        q = this.downstreamRelabelQuery(q)
      }
      return q
    },

    upstreamRelabelQueryHTTP: function(q) {
      return `label_replace(${q}, "upstream", "$1", "envoy_http_conn_manager_prefix", "upstream_(.*)_http")`
    },

    upstreamRelabelQueryTCP: function(q) {
      return `label_replace(${q}, "upstream", "$1", "envoy_tcp_prefix", "upstream_(.*)_tcp")`
    },

    downstreamRelabelQuery: function(q) {
      return `label_replace(${q}, "downstream", "$1", "local_cluster", "(.*)")`
    },

    groupBy: function(type) {
      if (type == "service") {
        return false
      }
      return type;
    },

    metricPrefixHTTP: function(type) {
      if (type == "downstream") {
        return "envoy_cluster_upstream_rq"
      }
      return "envoy_http_downstream_rq";
    },

    metricPrefixTCP: function(type) {
      if (type == "downstream") {
        return "envoy_cluster_upstream_cx"
      }
      return "envoy_tcp_downstream_cx";
    },

    fetchRPS: function(serviceName, type, options){
      var sel = this.makeHTTPSelector(serviceName, type)
      var subject = this.makeSubject(serviceName, type)
      var metricPfx = this.metricPrefixHTTP(type)
      var q = `sum(rate(${metricPfx}_completed{${sel}}[15m]))`
      return this.fetchStat(this.groupQueryHTTP(type, q),
        "RPS",
        `<b>${subject}</b> request rate averaged over the last 15 minutes`,
        shortNumStr,
        this.groupBy(type)
        )
    },

    fetchER: function(serviceName, type, options){
      var sel = this.makeHTTPSelector(serviceName, type)
      var subject = this.makeSubject(serviceName, type)
      var groupBy = ""
      if (type == "upstream") {
        groupBy += " by (envoy_http_conn_manager_prefix)"
      } else if (type == "downstream") {
        groupBy += " by (local_cluster)"
      }
      var metricPfx = this.metricPrefixHTTP(type)
      var q = `sum(rate(${metricPfx}_xx{${sel},envoy_response_code_class="5"}[15m]))${groupBy}/sum(rate(${metricPfx}_xx{${sel}}[15m]))${groupBy}`
      if (type == "upstream") {
        q = this.upstreamRelabelQueryHTTP(q)
      } else if (type == "downstream") {
        q = this.downstreamRelabelQuery(q)
      }
      return this.fetchStat(q,
        "ER",
        `Percentage of <b>${subject}</b> requests which were 5xx status over the last 15 minutes`,
        function(val){
          return shortNumStr(val)+"%"
        },
        this.groupBy(type)
        )
    },

    fetchPercentile: function(percentile, serviceName, type, options){
      var sel = this.makeHTTPSelector(serviceName, type)
      var subject = this.makeSubject(serviceName, type)
      var groupBy = "le"
      if (type == "upstream") {
        groupBy += ",envoy_http_conn_manager_prefix"
      } else if (type == "downstream") {
        groupBy += ",local_cluster"
      }
      var metricPfx = this.metricPrefixHTTP(type)
      var q = `histogram_quantile(${percentile/100}, sum by(${groupBy}) (rate(${metricPfx}_time_bucket{${sel}}[15m])))`
      if (type == "upstream") {
        q = this.upstreamRelabelQueryHTTP(q)
      } else if (type == "downstream") {
        q = this.downstreamRelabelQuery(q)
      }
      return this.fetchStat(q,
        `P${percentile}`,
        `<b>${subject}</b> ${percentile}th percentile request service time over the last 15 minutes`,
        shortTimeStr,
        this.groupBy(type)
        )
    },

    fetchConnRate: function(serviceName, type, options) {
      var sel = this.makeTCPSelector(serviceName, type)
      var subject = this.makeSubject(serviceName, type)
      var metricPfx = this.metricPrefixTCP(type)
      var q = `sum(rate(${metricPfx}_total{${sel}}[15m]))`
      return this.fetchStat(this.groupQueryTCP(type, q),
        "CR",
        `<b>${subject}</b> inbound TCP connections per second averaged over the last 15 minutes`,
        shortNumStr,
        this.groupBy(type)
        )
    },

    fetchServiceRx: function(serviceName, type, options) {
      var sel = this.makeTCPSelector(serviceName, type)
      var subject = this.makeSubject(serviceName, type)
      var metricPfx = this.metricPrefixTCP(type)
      var q = `8 * sum(rate(${metricPfx}_rx_bytes_total{${sel}}[15m]))`
      return this.fetchStat(this.groupQueryTCP(type, q),
        "RX",
        `<b>${subject}</b> received bits per second averaged over the last 15 minutes`,
        dataRateStr,
        this.groupBy(type)
        )
    },

    fetchServiceTx: function(serviceName, type, options) {
      var sel = this.makeTCPSelector(serviceName, type)
      var subject = this.makeSubject(serviceName, type)
      var metricPfx = this.metricPrefixTCP(type)
      var q = `8 * sum(rate(${metricPfx}_tx_bytes_total{${sel}}[15m]))`
      var self = this
      return this.fetchStat(this.groupQueryTCP(type, q),
        "TX",
        `<b>${subject}</b> transmitted bits per second averaged over the last 15 minutes`,
        dataRateStr,
        this.groupBy(type)
        )
    },

    fetchServiceNoRoute: function(serviceName, type, options) {
      var sel = this.makeTCPSelector(serviceName, type)
      var subject = this.makeSubject(serviceName, type)
      var metricPfx = this.metricPrefixTCP(type)
      var metric = "_no_route"
      if (type == "downstream") {
        metric = "_connect_fail"
      }
      var q = `sum(rate(${metricPfx}${metric}{${sel}}[15m]))`
      return this.fetchStat(this.groupQueryTCP(type, q),
        "NR",
        `<b>${subject}</b> unroutable (failed) connections per second averaged over the last 15 minutes`,
        shortNumStr,
        this.groupBy(type)
        )
    },

    fetchStat: function(promql, label, desc, formatter, groupBy) {
      if (!groupBy) {
        // If we don't have a grouped result and its just a single stat, return
        // no result as a zero not a missing stat.
        promql += " OR on() vector(0)";
      }
      //console.log(promql)
      var params = {
        query: promql,
        time: (new Date).getTime()/1000
      }
      return this.httpGet("/api/v1/query", params).then(function(response){
        if (!groupBy) {
          // Not grouped, expect just one stat value return that
          var v = parseFloat(response.data.result[0].value[1])
          return {
            label: label,
            desc: desc,
            value: formatter(v)
          }
        }

        var data = {};
        for (var i = 0; i < response.data.result.length; i++) {
          var res = response.data.result[i];
          var v = parseFloat(res.value[1]);
          var groupName = res.metric[groupBy];
          data[groupName] = {
            label: label,
            desc: desc.replace('{{GROUP}}', groupName),
            value: formatter(v)
          }
        }
        return data;
      }, function(xhr){
        // Failure. log to console and return an blank result for now.
        console.log("ERROR: failed to fetch stat", label, xhr.responseText)
        return {}
      })
    },

    fetchSeries: function(promql, options) {
      var now = (new Date()).getTime()/1000;
      var startT = now - (15*60);
      var params = {
        query: promql,
        start: startT,
        end: now,
        step: "10s",
        timeout: "8s"
      }
      return this.httpGet("/api/v1/query_range", params)
    },

    httpGet: function(path, params) {
      var xhr = new XMLHttpRequest();
      var self = this
      return new Promise(function(resolve, reject){
        xhr.onreadystatechange = function(){
          if (xhr.readyState !== 4) return;

          if (xhr.status == 200) {
            // Attempt to parse response as JSON and return the object
            var o = JSON.parse(xhr.responseText)
            resolve(o)
          }
          reject(xhr)
        }

        var url = self.baseURL()+path;
        if (params) {
          var qs = Object.keys(params).
          map(function(key){
            return encodeURIComponent(key)+"="+encodeURIComponent(params[key])
          }).
          join("&")
          url = url+"?"+qs
        }
        xhr.open("GET", url, true);
        xhr.send();
      });
    },

    baseURL: function() {
      // TODO support configuring a direct Prometheus via
      // metrics_provider_options_json.
      return "/v1/internal/ui/metrics-proxy"
    }
  }

  // Helper functions
  function shortNumStr(n) {
    if (n < 1e3) {
      if (Number.isInteger(n)) return ""+n
      return n.toPrecision(2)
    }
    if (n >= 1e3 && n < 1e6) return +(n / 1e3).toFixed(1) + "k";
    if (n >= 1e6 && n < 1e9) return +(n / 1e6).toFixed(1) + "m";
    if (n >= 1e9 && n < 1e12) return +(n / 1e9).toFixed(1) + "b";
    if (n >= 1e12) return +(n / 1e12).toFixed(1) + "t";
  }

  function dataRateStr(n) {
    return shortNumStr(n) + "bps";
  }

  function shortTimeStr(n) {
    if (n < 1e3) return Math.round(n) + "ms";

    var secs = n / 1e3
    if (secs < 60) return secs.toFixed(1) + "s"

    var mins = secs/60
    if (mins < 60) return mins.toFixed(1) + "m"

    var hours = mins/60
    if (hours < 24) return hours.toFixed(1) + "h"

    var days = hours/24
    return days.toFixed(1) + "d"
  }


  consul.registerMetricsProvider("prometheus", prometheusProvider)

}());