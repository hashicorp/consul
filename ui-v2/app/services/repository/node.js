import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';
const modelName = 'node';
export default RepositoryService.extend({
  coordinates: service('repository/coordinate'),
  getModelName: function() {
    return modelName;
  },
  findBySlug: function(slug, dc) {
    return this._super(...arguments).then(node => {
      return get(this, 'coordinates')
        .findAllByDatacenter(dc)
        .then(function(res) {
          node.Coordinates = res;
          return node;
        });
    });
  },
});
