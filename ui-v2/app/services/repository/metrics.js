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
  error: null,

  init: function() {
    this._super(...arguments);
    const uiCfg = this.cfg.get();
    // Inject whether or not the proxy is enabled as an option into the opaque
    // JSON options the user provided.
    const opts = uiCfg.metrics_provider_options || {};
    opts.metrics_proxy_enabled = uiCfg.metrics_proxy_enabled;
    // Inject the base app URL
    const provider = uiCfg.metrics_provider || 'prometheus';

    try {
      this.provider = window.consul.getMetricsProvider(provider, opts);
    } catch(e) {
      this.error = new Error(`metrics provider not initialized: ${e}`);
      // Show the user the error once for debugging their provider outside UI
      // Dev.
      console.error(this.error);
    }
  },

  findServiceSummary: function(protocol, slug, dc, nspace, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    const promises = [
      // TODO: support namespaces in providers
      this.provider.serviceRecentSummarySeries(slug, protocol, {}),
      this.provider.serviceRecentSummaryStats(slug, protocol, {}),
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
    return this.provider.upstreamRecentSummaryStats(slug, {}).then(function(result) {
      result.meta = meta;
      return result;
    });
  },

  findDownstreamSummary: function(slug, dc, nspace, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    return this.provider.downstreamRecentSummaryStats(slug, {}).then(function(result) {
      result.meta = meta;
      return result;
    });
  }
});
