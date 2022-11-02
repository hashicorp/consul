import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'discovery-chain';
const ERROR_MESH_DISABLED = 'Connect must be enabled in order to use this endpoint';
export default class DiscoveryChainService extends RepositoryService {
  @service('repository/dc')
  dcs;

  getModelName() {
    return modelName;
  }

  @dataSource('/:partition/:ns/:dc/discovery-chain/:id')
  findBySlug(params, configuration = {}) {
    // peekAll and find is fine here as datacenter count should be relatively
    // low, and DCs are the top bucket (when talking dc's partitions, nspaces)
    const datacenter = this.dcs.peekAll().findBy('Name', params.dc);
    if (typeof datacenter !== 'undefined' && !get(datacenter, 'MeshEnabled')) {
      return Promise.resolve();
    }
    return super.findBySlug(...arguments).catch((e) => {
      const code = get(e, 'errors.firstObject.status');
      const body = (get(e, 'errors.firstObject.detail') || '').trim();
      switch (code) {
        case '500':
          if (typeof datacenter !== 'undefined' && body.endsWith(ERROR_MESH_DISABLED)) {
            set(datacenter, 'MeshEnabled', false);
          }
          return;
        default:
          throw e;
      }
    });
  }
}
