import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/nspace';

const modelName = 'nspace';
export default class NspaceEnabledService extends RepositoryService {
  @service('router') router;
  @service('container') container;
  @service('env') env;

  @service('settings') settings;

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
    if (!this.env.var('CONSUL_ACLS_ENABLED')) {
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

  getActive(paramsNspace) {
    return this.settings
      .findBySlug('nspace')
      .then(function(nspace) {
        // If we can't figure out the nspace from the URL use
        // the previously saved nspace and if thats not there
        // then just use default
        return paramsNspace || nspace || 'default';
      })
      .then(nspace => this.settings.persist({ nspace: nspace }))
      .then(function(item) {
        return {
          Name: item.nspace,
        };
      });
  }
}
