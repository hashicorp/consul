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
      if (nodes.length === 0) {
        // TODO: Add an store.error("404", "message") or similar
        // or move all this to serializer
        const e = new Error();
        e.errors = [
          {
            status: '404',
            title: 'Not found',
          },
        ];
        throw e;
      }
      const service = get(nodes, 'firstObject');
      const tags = nodes
        .reduce(function(prev, item) {
          return prev.concat(get(item, 'Service.Tags') || []);
        }, [])
        .uniq();
      set(service, 'Tags', tags);
      set(service, 'Nodes', nodes);
      set(service, 'meta', get(item, 'meta'));
      return service;
    });
  },
  findInstanceBySlug: function(id, node, slug, dc, configuration) {
    return this.findBySlug(slug, dc, configuration).then(function(item) {
      // Loop through all the service instances and pick out the one
      // that has the same service id AND node name
      // node names are unique per datacenter
      const i = item.Nodes.findIndex(function(item) {
        return item.Service.ID === id && item.Node.Node === node;
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
        set(service, 'meta', get(item, 'meta'));
        return service;
      }
      // TODO: Add an store.error("404", "message") or similar
      // or move all this to serializer
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
