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
  findInstanceBySlug: function(id, slug, dc, configuration) {
    return this.findBySlug(slug, dc, configuration).then(function(item) {
      const i = item.Nodes.findIndex(function(item) {
        return item.Service.ID === id;
      });
      if (i !== -1) {
        const service = item.Nodes[i].Service;
        service.Node = item.Nodes[i].Node;
        service.ServiceChecks = item.Nodes[i].Checks.filter(function(item) {
          return item.ServiceID != '';
        });
        service.NodeChecks = item.Nodes[i].Checks.filter(function(item) {
          return item.ServiceID == '';
        });
        return service;
      }
      // TODO: Add an store.error("404", "message") or similar
      const e = new Error();
      e.errors = [
        {
          status: '404',
          title: 'Unable to find instance',
        },
      ];
      throw e;
    });
  },
});
