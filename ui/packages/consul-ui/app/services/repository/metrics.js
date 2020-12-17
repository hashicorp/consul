import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';

// CONSUL_METRICS_POLL_INTERVAL controls how long between each poll to the
// metrics provider

export default class MetricsService extends RepositoryService {
  @service('ui-config') config;
  @service('env') env;
  @service('client/http') client;

  error = null;

  init() {
    super.init(...arguments);
    const config = this.config.get();
    // Inject whether or not the proxy is enabled as an option into the opaque
    // JSON options the user provided.
    const opts = config.metrics_provider_options || {};
    opts.metrics_proxy_enabled = config.metrics_proxy_enabled;
    // Inject a convenience function for dialing through the metrics proxy.
    opts.fetch = (path, params) =>
      this.client.fetchWithToken(`/v1/internal/ui/metrics-proxy${path}`, params);
    // Inject the base app URL
    const provider = config.metrics_provider || 'prometheus';

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
      this.provider.serviceRecentSummarySeries(slug, dc, nspace, protocol, {}),
      this.provider.serviceRecentSummaryStats(slug, dc, nspace, protocol, {}),
    ];
    return Promise.all(promises).then(results => {
      return {
        meta: {
          interval: this.env.var('CONSUL_METRICS_POLL_INTERVAL') || 10000,
        },
        series: results[0],
        stats: results[1].stats,
      };
    });
  }

  findUpstreamSummary(slug, dc, nspace, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    return this.provider.upstreamRecentSummaryStats(slug, dc, nspace, {}).then(result => {
      result.meta = {
        interval: this.env.var('CONSUL_METRICS_POLL_INTERVAL') || 10000,
      };
      return result;
    });
  }

  findDownstreamSummary(slug, dc, nspace, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    return this.provider.downstreamRecentSummaryStats(slug, dc, nspace, {}).then(result => {
      result.meta = {
        interval: this.env.var('CONSUL_METRICS_POLL_INTERVAL') || 10000,
      };
      return result;
    });
  }
}
