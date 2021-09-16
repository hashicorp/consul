import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { getOwner } from '@ember/application';
import { match } from 'consul-ui/decorators/data-source';
import { singularize } from 'ember-inflector';

export default class HttpService extends Service {
  @service('repository/dc') datacenters;
  @service('repository/dc') datacenter;
  @service('repository/kv') kvs;
  @service('repository/kv') kv;
  @service('repository/node') leader;
  @service('repository/service') gateways;
  @service('repository/service-instance') 'proxy-service-instance';
  @service('repository/proxy') 'proxy-instance';
  @service('repository/nspace') namespaces;
  @service('repository/nspace') namespace;
  @service('repository/metrics') metrics;
  @service('repository/oidc-provider') oidc;
  @service('ui-config') 'ui-config';
  @service('ui-config') notfound;

  @service('data-source/protocols/http/blocking') type;

  source(src, configuration) {
    const [, , , , model] = src.split('/');
    const owner = getOwner(this);
    const route = match(src);
    const find = route.cb(route.params, owner);

    const repo = this[model] || owner.lookup(`service:repository/${singularize(model)}`);
    if (typeof repo.reconcile === 'function') {
      configuration.createEvent = function(result = {}, configuration) {
        const event = {
          type: 'message',
          data: result,
        };
        const meta = get(event, 'data.meta') || {};
        if (typeof meta.range === 'undefined') {
          repo.reconcile(meta);
        }
        return event;
      };
    }
    return this.type.source(find, configuration);
  }
}
