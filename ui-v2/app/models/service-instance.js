import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import { computed } from '@ember/object';
import { or, filter, alias } from '@ember/object/computed';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Node.Node,Service.ID';

export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Datacenter: attr('string'),
  // ProxyInstance is the ember-data model relationship
  ProxyInstance: belongsTo('Proxy'),
  // Proxy is the actual JSON api response
  Proxy: attr(),
  Node: attr(),
  Service: attr(),
  Checks: attr(),
  SyncTime: attr('number'),
  meta: attr(),
  Name: or('Service.ID', 'Service.Service'),
  Tags: alias('Service.Tags'),
  Meta: alias('Service.Meta'),
  Namespace: alias('Service.Namespace'),
  ExternalSources: computed('Service.Meta', function() {
    const sources = Object.entries(this.Service.Meta || {})
      .filter(([key, value]) => key === 'external-source')
      .map(([key, value]) => {
        return value;
      });
    return [...new Set(sources)];
  }),
  ServiceChecks: filter('Checks.[]', function(item, i, arr) {
    return item.ServiceID !== '';
  }),
  NodeChecks: filter('Checks.[]', function(item, i, arr) {
    return item.ServiceID === '';
  }),
  Status: computed('ChecksPassing', 'ChecksWarning', 'ChecksCritical', function() {
    switch (true) {
      case this.ChecksCritical.length !== 0:
        return 'critical';
      case this.ChecksWarning.length !== 0:
        return 'warning';
      case this.ChecksPassing.length !== 0:
        return 'passing';
      default:
        return 'empty';
    }
  }),
  ChecksPassing: computed('Checks.[]', function() {
    return this.Checks.filter(item => item.Status === 'passing');
  }),
  ChecksWarning: computed('Checks.[]', function() {
    return this.Checks.filter(item => item.Status === 'warning');
  }),
  ChecksCritical: computed('Checks.[]', function() {
    return this.Checks.filter(item => item.Status === 'critical');
  }),
  PercentageChecksPassing: computed('Checks.[]', 'ChecksPassing', function() {
    return (this.ChecksPassing.length / this.Checks.length) * 100;
  }),
  PercentageChecksWarning: computed('Checks.[]', 'ChecksWarning', function() {
    return (this.ChecksWarning.length / this.Checks.length) * 100;
  }),
  PercentageChecksCritical: computed('Checks.[]', 'ChecksCritical', function() {
    return (this.ChecksCritical.length / this.Checks.length) * 100;
  }),
});
