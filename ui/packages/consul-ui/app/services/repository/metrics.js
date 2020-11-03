import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { env } from 'consul-ui/env';

// meta is used by DataSource to configure polling. The interval controls how
// long between each poll to the metrics provider. TODO - make this configurable
// in the UI settings.
const meta = {
  interval: env('CONSUL_METRICS_POLL_INTERVAL') || 10000,
};

export default RepositoryService.extend({
  cfg: service('ui-config'),
  settings: service('settings'),
  error: null,

  init: function() {
    this._super(...arguments);
    const uiCfg = this.cfg.get();
    // Inject whether or not the proxy is enabled as an option into the opaque
    // JSON options the user provided.
    const opts = uiCfg.metrics_provider_options || {};
    opts.metrics_proxy_enabled = uiCfg.metrics_proxy_enabled;
    // Inject a convenience function for dialing through the metrics proxy.
    opts.httpGet = (path, params) => {
      return this.httpGet(path, params);
    };
    // Inject the base app URL
    const provider = uiCfg.metrics_provider || 'prometheus';

    try {
      this.provider = window.consul.getMetricsProvider(provider, opts);
    } catch (e) {
      this.error = new Error(`metrics provider not initialized: ${e}`);
      // Show the user the error once for debugging their provider outside UI
      // Dev.
      console.error(this.error); // eslint-disable-line no-console
    }
  },

  httpGet: function(path, params) {
    var xhr = new XMLHttpRequest();
    var self = this;
    return self.settings.findBySlug('token').then(token => {
      var tokenValue = typeof token.SecretID === 'undefined' ? '' : token.SecretID;

      return new Promise(function(resolve, reject) {
        xhr.onreadystatechange = function() {
          if (xhr.readyState !== 4) return;

          if (xhr.status == 200) {
            // Attempt to parse response as JSON and return the object
            var o = JSON.parse(xhr.responseText);
            resolve(o);
          }
          const e = new Error(xhr.statusText);
          e.statusCode = xhr.status;
          reject(e);
        };

        var url = '/v1/internal/ui/metrics-proxy' + path;
        if (params) {
          var qs = Object.keys(params)
            .map(function(key) {
              return encodeURIComponent(key) + '=' + encodeURIComponent(params[key]);
            })
            .join('&');
          url = url + '?' + qs;
        }
        xhr.open('GET', url, true);
        if (tokenValue) {
          xhr.setRequestHeader('X-Consul-Token', tokenValue);
        }
        xhr.send();
      });
    });
  },

  findServiceSummary: function(protocol, slug, dc, nspace, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    const promises = [
      this.provider.serviceRecentSummarySeries(dc, nspace, slug, protocol, {}),
      this.provider.serviceRecentSummaryStats(dc, nspace, slug, protocol, {}),
    ];
    return Promise.all(promises).then(function(results) {
      return {
        meta: meta,
        series: results[0],
        stats: results[1].stats,
      };
    });
  },

  findUpstreamSummary: function(slug, dc, nspace, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    return this.provider.upstreamRecentSummaryStats(dc, nspace, slug, {}).then(function(result) {
      result.meta = meta;
      return result;
    });
  },

  findDownstreamSummary: function(slug, dc, nspace, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    return this.provider.downstreamRecentSummaryStats(dc, nspace, slug, {}).then(function(result) {
      result.meta = meta;
      return result;
    });
  },
});
