import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { env } from 'consul-ui/env';
import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/nspace';

const modelName = 'nspace';
export default class EnabledService extends RepositoryService {
  @service('router')
  router;

  @service('settings')
  settings;

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  getSlugKey() {
    return SLUG_KEY;
  }

  getModelName() {
    return modelName;
  }

  findAll(params, configuration = {}) {
    const query = {};
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), query);
  }

  authorize(dc, nspace) {
    if (!env('CONSUL_ACLS_ENABLED')) {
      return Promise.resolve([
        {
          Resource: 'operator',
          Access: 'write',
          Allow: true,
        },
      ]);
    }
    return this.store.authorize(this.getModelName(), { dc: dc, ns: nspace }).catch(function(e) {
      return [];
    });
  }

  getActive() {
    let routeParams = {};
    // this is only populated before the model hook as fired,
    // it is then deleted after the model hook has finished
    const infos = get(this, 'router._router.currentState.router.activeTransition.routeInfos');
    if (typeof infos !== 'undefined') {
      infos.forEach(function(item) {
        Object.keys(item.params).forEach(function(prop) {
          routeParams[prop] = item.params[prop];
        });
      });
    } else {
      // this is only populated after the model hook has finished
      //
      const current = get(this, 'router.currentRoute');
      if (current) {
        const nspacedRoute = current.find(function(item, i, arr) {
          return item.paramNames.includes('nspace');
        });
        if (typeof nspacedRoute !== 'undefined') {
          routeParams.nspace = nspacedRoute.params.nspace;
        }
      }
    }
    return this.settings
      .findBySlug('nspace')
      .then(function(nspace) {
        // If we can't figure out the nspace from the URL use
        // the previously saved nspace and if thats not there
        // then just use default
        return routeParams.nspace || nspace || '~default';
      })
      .then(nspace => this.settings.persist({ nspace: nspace }))
      .then(function(item) {
        return {
          Name: item.nspace.substr(1),
        };
      });
  }
}
