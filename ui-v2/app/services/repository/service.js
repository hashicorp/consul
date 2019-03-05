import RepositoryService from 'consul-ui/services/repository';
import { get, set } from '@ember/object';
const modelName = 'service';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  findBySlug: function(slug, dc) {
    return this._super(...arguments).then(function(item) {
        const nodes = get(item, 'Nodes');
        const service = get(nodes, 'firstObject');
        const tags = nodes
          .reduce(function(prev, item) {
            return prev.concat(get(item, 'Service.Tags') || []);
          }, [])
          .uniq();
        set(service, 'Tags', tags);
        set(service, 'Nodes', nodes);
        return service;
      });
  },
});
