import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/nspace';
import { runInDebug } from '@ember/debug';

const findActiveNspace = function(nspaces, nspace) {
  let found = nspaces.find(function(item) {
    return item.Name === nspace.Name;
  });
  if (typeof found === 'undefined') {
    runInDebug(_ =>
      console.info(`${nspace.Name} not found in [${nspaces.map(item => item.Name).join(', ')}]`)
    );
    // if we can't find the nspace that was specified try default
    found = nspaces.find(function(item) {
      return item.Name === 'default';
    });
    // if there is no default just choose the first
    if (typeof found === 'undefined') {
      found = nspaces[0];
    }
  }
  return found;
};
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

  async getActive(paramsNspace = '') {
    const nspaces = this.store.peekAll('nspace').toArray();
    if (paramsNspace.length === 0) {
      const token = await this.settings.findBySlug('token');
      paramsNspace = token.Namespace || 'default';
    }
    // if there is only 1 namespace then use that, otherwise find the
    // namespace object that corresponds to the active one
    return nspaces.length === 1 ? nspaces[0] : findActiveNspace(nspaces, { Name: paramsNspace });
  }
}
