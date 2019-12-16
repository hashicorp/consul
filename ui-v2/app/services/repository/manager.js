import Service, { inject as service } from '@ember/service';

export default Service.extend({
  // TODO: Temporary repo list here
  service: service('repository/service'),
  services: service('repository/service'),
  node: service('repository/node'),
  nodes: service('repository/node'),
  session: service('repository/session'),
  proxy: service('repository/proxy'),

  fromURI: function(src, filter) {
    let temp = src.split('/');
    temp.shift();
    const dc = temp.shift();
    const model = temp.shift();
    const repo = this[model];

    // TODO: if something is filtered we shouldn't reconcile things
    // custom createEvent, here used to reconcile the ember-data store for each tick
    // ideally we wouldn't do this here, but we handily have access to the repo here
    const obj = {
      reconcile: function(result = {}, configuration) {
        const event = {
          type: 'message',
          data: result,
        };
        // const meta = get(event.data || {}, 'meta') || {};
        repo.reconcile(event.data.meta);
        return event;
      },
    };
    let slug = temp.join('/');
    let id, node;
    switch (model) {
      // plurals / listings
      case 'services':
      case 'nodes':
        obj.find = function(configuration) {
          return repo.findAllByDatacenter(dc, configuration);
        };
        break;
      // weirder ones
      case 'service-instance':
        temp = slug.split('/');
        id = temp[0];
        node = temp[1];
        slug = temp[2];
        obj.find = function(configuration) {
          return repo.findInstanceBySlug(id, node, slug, dc, configuration);
        };
        break;
      case 'proxy':
        temp = slug.split('/');
        id = temp[0];
        node = temp[1];
        slug = temp[2];
        obj.find = function(configuration) {
          return repo.findInstanceBySlug(id, node, slug, dc, configuration);
        };
        break;
      // commoner slugs
      default:
        obj.find = function(configuration) {
          return repo.findBySlug(slug, dc, configuration);
        };
    }
    return obj;
  },
});
