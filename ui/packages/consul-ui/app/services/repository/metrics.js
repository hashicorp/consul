import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

// CONSUL_METRICS_POLL_INTERVAL controls how long between each poll to the
// metrics provider
export default class MetricsService extends RepositoryService {
  @service('ui-config') config;
  @service('env') env;
  @service('client/http') client;

  error = null;

  getModelName() {
    return 'metrics';
  }

  init() {
    super.init(...arguments);
    // TODO: this flow should be be async, then can just use either get or a DataSource
    const config = this.config.getSync();
    // Inject whether or not the proxy is enabled as an option into the opaque
    // JSON options the user provided.
    const opts = config.metrics_provider_options || {};
    opts.metrics_proxy_enabled = config.metrics_proxy_enabled;
    // Inject the base app URL
    const provider = config.metrics_provider || 'prometheus';
    // Inject a convenience function for dialing through the metrics proxy.
    opts.fetch = (path, params) =>
      this.client.fetchWithToken(`/v1/internal/ui/metrics-proxy${path}`, params);

    try {
      this.provider = window.consul.getMetricsProvider(provider, opts);
    } catch (e) {
      this.error = new Error(`metrics provider not initialized: ${e}`);
      // Show the user the error once for debugging their provider outside UI
      // Dev.
      console.error(this.error); // eslint-disable-line no-console
    }
  }

  @dataSource('/:partition/:ns/:dc/metrics/summary-for-service/:slug/:protocol')
  findServiceSummary(params, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    const promises = [
      this.provider.serviceRecentSummarySeries(
        params.slug,
        params.dc,
        params.ns,
        params.protocol,
        {}
      ),
      this.provider.serviceRecentSummaryStats(
        params.slug,
        params.dc,
        params.ns,
        params.protocol,
        {}
      ),
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

  @dataSource('/:partition/:ns/:dc/metrics/upstream-summary-for-service/:slug/:protocol')
  findUpstreamSummary(params, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    return this.provider
      .upstreamRecentSummaryStats(params.slug, params.dc, params.ns, {})
      .then(result => {
        result.meta = {
          interval: this.env.var('CONSUL_METRICS_POLL_INTERVAL') || 10000,
        };
        return result;
      });
  }

  @dataSource('/:partition/:ns/:dc/metrics/downstream-summary-for-service/:slug/:protocol')
  findDownstreamSummary(params, configuration = {}) {
    if (this.error) {
      return Promise.reject(this.error);
    }
    return this.provider
      .downstreamRecentSummaryStats(params.slug, params.dc, params.ns, {})
      .then(result => {
        result.meta = {
          interval: this.env.var('CONSUL_METRICS_POLL_INTERVAL') || 10000,
        };
        return result;
      });
  }
}
