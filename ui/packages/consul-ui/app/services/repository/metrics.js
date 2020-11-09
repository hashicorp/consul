import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import { env } from 'consul-ui/env';

// meta is used by DataSource to configure polling. The interval controls how
// long between each poll to the metrics provider. TODO - make this configurable
// in the UI settings.
const meta = {
  interval: env('CONSUL_METRICS_POLL_INTERVAL') || 10000,
};

export default class MetricsService extends RepositoryService {
  @service('ui-config')
  cfg;

  @service('client/http')
  client;

  error = null;

  init() {
    super.init(...arguments);
    const uiCfg = this.cfg.get();
    // Inject whether or not the proxy is enabled as an option into the opaque
    // JSON options the user provided.
    const opts = uiCfg.metrics_provider_options || {};
    opts.metrics_proxy_enabled = uiCfg.metrics_proxy_enabled;
    // Inject a convenience function for dialing through the metrics proxy.
    opts.fetch = (path, params) =>
      this.client.fetchWithToken(`/v1/internal/ui/metrics-proxy${path}`, params);
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
  }

  findServiceSummary(protocol, slug, dc, nspace, configuration = {}) {
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
  }

  findUpstreamSummary(slug, dc, nspace, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    return this.provider.upstreamRecentSummaryStats(dc, nspace, slug, {}).then(function(result) {
      result.meta = meta;
      return result;
    });
  }

  findDownstreamSummary(slug, dc, nspace, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    return this.provider.downstreamRecentSummaryStats(dc, nspace, slug, {}).then(function(result) {
      result.meta = meta;
      return result;
    });
  }
}
