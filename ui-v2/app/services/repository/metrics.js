import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';

export default RepositoryService.extend({

  cfg: service('ui-config'),

  init: function(){
    this._super(...arguments);
    const uiCfg = this.cfg.get()
    // Inject whether or not the proxy is enabled as an option into the opaque
    // JSON options the user provided.
    const opts = uiCfg.metrics_provider_options || {};
    opts.metrics_proxy_enabled = uiCfg.metrics_proxy_enabled ;
    const provider = uiCfg.metrics_provider || 'prometheus';
    this.provider = window.consul.getMetricsProvider(provider, opts);
  },

  findServiceSummary: function(slug, dc, nspace, configuration = {}) {
    const promises = [
      // TODO: support namespaces in providers
      // TODO: work out how to depend on the actual service to figure out it's protocol
      this.provider.serviceRecentSummarySeries(slug, "tcp", {}),
      this.provider.serviceRecentSummaryStats(slug, "tcp", {})
    ];
    return Promise.all(promises).then(function(results){
      return {
        meta: {
          // Arbitrary value expected by data-source
          cursor: 1
        },
        series: results[0].series,
        stats: results[1].stats
      }
    })
  },

  findUpstreamSummary: function(slug, dc, nspace, configuration = {}) {
    return this.provider.upstreamRecentSummaryStats(slug, {}).then(function(result){
      result.meta = {cursor: 1};
      return result;
    })
  },

  findDownstreamSummary: function(slug, dc, nspace, configuration = {}) {
    return this.provider.downstreamRecentSummaryStats(slug, {}).then(function(result){
      result.meta = {cursor: 1};
      return result;
    })
  }

});