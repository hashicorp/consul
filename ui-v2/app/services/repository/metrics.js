import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';

// meta is used by DataSource to configure polling. The cursor is fake since we
// aren't really doing blocking queries. The pollInterval controls how long
// between each poll to the metrics provider.
// TODO - make this configurable in the UI settings.
const meta = {
  cursor: 1,
  pollInterval: 10000,
}

export default RepositoryService.extend({

  cfg: service('ui-config'),

  init: function(){
    this._super(...arguments);
    const uiCfg = this.cfg.get()
    // Inject whether or not the proxy is enabled as an option into the opaque
    // JSON options the user provided.
    const opts = uiCfg.metrics_provider_options || {};
    opts.metrics_proxy_enabled = uiCfg.metrics_proxy_enabled;
    // Inject the base app URL
    const provider = uiCfg.metrics_provider || 'prometheus';
    this.provider = window.consul.getMetricsProvider(provider, opts);
  },

  findServiceSummary: function(protocol, slug, dc, nspace, configuration = {}) {
    const promises = [
      // TODO: support namespaces in providers
      this.provider.serviceRecentSummarySeries(slug, protocol, {}),
      this.provider.serviceRecentSummaryStats(slug, protocol, {})
    ];
    return Promise.all(promises).then(function(results){
      return {
        meta: meta,
        series: results[0].series,
        stats: results[1].stats
      }
    })
  },

  findUpstreamSummary: function(slug, dc, nspace, configuration = {}) {
    return this.provider.upstreamRecentSummaryStats(slug, {}).then(function(result){
      result.meta = meta;
      return result;
    })
  },

  findDownstreamSummary: function(slug, dc, nspace, configuration = {}) {
    return this.provider.downstreamRecentSummaryStats(slug, {}).then(function(result){
      result.meta = meta;
      return result;
    })
  }

});