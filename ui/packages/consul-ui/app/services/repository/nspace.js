import { inject as service } from '@ember/service';
import { runInDebug } from '@ember/debug';
import RepositoryService, { softDelete } from 'consul-ui/services/repository';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/nspace';
import dataSource from 'consul-ui/decorators/data-source';

import { defaultChangeset as changeset } from 'consul-ui/utils/form/builder';

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
export default class NspaceService extends RepositoryService {
  @service('router') router;
  @service('container') container;
  @service('env') env;
  @service('form') form;

  @service('settings') settings;
  @service('repository/permission') permissions;

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  getSlugKey() {
    return SLUG_KEY;
  }

  getModelName() {
    return modelName;
  }

  @dataSource('/:partition/:ns/:dc/namespaces')
  async findAll() {
    if (!this.permissions.can('use nspaces')) {
      return [];
    }
    return super.findAll(...arguments).catch(() => []);
  }

  @dataSource('/:partition/:ns/:dc/namespace/:id')
  async findBySlug(params) {
    let item;
    if (params.id === '') {
      item = await this.create({
        Datacenter: params.dc,
        Partition: params.partition,
        ACLs: {
          PolicyDefaults: [],
          RoleDefaults: [],
        },
      });
    } else {
      item = await super.findBySlug(...arguments);
    }
    return changeset(item);
  }

  remove(item) {
    return softDelete(this, item);
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
    if (this.permissions.can('use nspaces')) {
      return {
        Name: 'default',
      };
    }
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
