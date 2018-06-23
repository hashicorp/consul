import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';

export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return get(this, 'store')
      .query('service', { dc: dc })
      .then(function(items) {
        return items.forEach(function(item) {
          set(item, 'Datacenter', dc);
        });
      });
  },
  findBySlug: function(slug, dc) {
    return get(this, 'store')
      .queryRecord('service', {
        id: slug,
        dc: dc,
      })
      .then(function(item) {
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
