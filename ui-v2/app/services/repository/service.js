import RepositoryService from 'consul-ui/services/repository';
import { get, set } from '@ember/object';
const modelName = 'service';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  shouldReconcile: function(method) {
    switch (method) {
      case 'findGatewayBySlug':
        return false;
    }
    return this._super(...arguments);
  },
  findBySlug: function(slug, dc) {
    return this._super(...arguments).then(function(item) {
      // TODO: Move this to the Serializer
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
      // TODO: Use [...new Set()] instead of uniq
      const tags = nodes
        .reduce(function(prev, item) {
          return prev.concat(get(item, 'Service.Tags') || []);
        }, [])
        .uniq();
      set(service, 'Tags', tags);
      set(service, 'Nodes', nodes);
      set(service, 'meta', get(item, 'meta'));
      set(service, 'Namespace', get(item, 'Namespace'));
      return service;
    });
  },
  findInstanceBySlug: function(id, node, slug, dc, nspace, configuration) {
    return this.findBySlug(slug, dc, nspace, configuration).then(function(item) {
      // TODO: Move this to the Serializer
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
        set(service, 'Namespace', get(item, 'Namespace'));
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
  findGatewayBySlug: function(slug, dc, nspace, configuration) {
    const query = {
      dc: dc,
      ns: nspace,
      gateway: slug,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
    }
    return this.store.query(this.getModelName(), query);
  },
});
