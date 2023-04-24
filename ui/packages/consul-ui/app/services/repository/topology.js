import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import { get, set } from '@ember/object';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'topology';
const ERROR_MESH_DISABLED = 'Connect must be enabled in order to use this endpoint';

export default class TopologyService extends RepositoryService {
  @service('repository/dc')
  dcs;

  getModelName() {
    return modelName;
  }

  @dataSource('/:partition/:ns/:dc/topology/:id/:kind')
  findBySlug(params, configuration = {}) {
    const datacenter = this.dcs.peekOne(params.dc);
    if (datacenter !== null && !get(datacenter, 'MeshEnabled')) {
      return Promise.resolve();
    }
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.store.queryRecord(this.getModelName(), params).catch(e => {
      const code = get(e, 'errors.firstObject.status');
      const body = (get(e, 'errors.firstObject.detail') || '').trim();
      switch (code) {
        case '500':
          if (datacenter !== null && body.endsWith(ERROR_MESH_DISABLED)) {
            set(datacenter, 'MeshEnabled', false);
          }
          return;
        default:
          throw e;
      }
    });
  }
}
