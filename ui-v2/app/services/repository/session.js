import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';

const modelName = 'session';
export default RepositoryService.extend({
  store: service('store'),
  getModelName: function() {
    return modelName;
  },
  findByNode: function(node, dc) {
    return get(this, 'store').query(this.getModelName(), {
      id: node,
      dc: dc,
    });
  },
  // TODO: Why Key? Probably should be findBySlug like the others
  findByKey: function(slug, dc) {
    return this.findBySlug(slug, dc);
  },
});
