import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import { filter, alias } from '@ember/object/computed';

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
  Tags: alias('Service.Tags'),
  Meta: alias('Service.Meta'),
  Namespace: alias('Service.Namespace'),
  ServiceChecks: filter('Checks', function(item, i, arr) {
    return item.ServiceID !== '';
  }),
  NodeChecks: filter('Checks', function(item, i, arr) {
    return item.ServiceID === '';
  }),
});
