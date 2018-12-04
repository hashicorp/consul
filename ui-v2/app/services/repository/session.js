import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';

const modelName = 'session';
export default RepositoryService.extend({
  store: service('store'),
  getModelName: function() {
    return modelName;
  },
  findByNode: function(node, dc, configuration = {}) {
    const query = {
      id: node,
      dc: dc,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
    }
    return get(this, 'store').query(this.getModelName(), query);
  },
  // TODO: Why Key? Probably should be findBySlug like the others
  findByKey: function(slug, dc) {
    return this.findBySlug(slug, dc);
  },
});
