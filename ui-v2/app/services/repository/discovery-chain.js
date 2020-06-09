import RepositoryService from 'consul-ui/services/repository';
import { get, set } from '@ember/object';

const modelName = 'discovery-chain';
const ERROR_MESH_DISABLED = 'Connect must be enabled in order to use this endpoint';
export default RepositoryService.extend({
  meshEnabled: true,
  getModelName: function() {
    return modelName;
  },
  findBySlug: function() {
    if (!this.meshEnabled) {
      return Promise.resolve();
    }
    return this._super(...arguments).catch(e => {
      const code = get(e, 'errors.firstObject.status');
      const body = get(e, 'errors.firstObject.detail').trim();
      switch (code) {
        case '500':
          if (body === ERROR_MESH_DISABLED) {
            set(this, 'meshEnabled', false);
          }
          return;
        default:
          return;
      }
    });
  },
});
