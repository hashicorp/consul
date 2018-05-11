import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';
import Error from '@ember/error';

export default Service.extend({
  store: service('store'),
  settings: service('settings'),
  findBySlug: function(name, items) {
    if (name != null) {
      const item = items.findBy('Name', name);
      if (item) {
        return get(this, 'settings')
          .persist({ dc: get(item, 'Name') })
          .then(function() {
            // TODO: create a model
            return { Name: get(item, 'Name') };
          });
      }
    }
    const e = new Error();
    e.status = '404';
    e.detail = 'Page not found';
    return Promise.reject({ errors: [e] });
  },
  getActive: function(name, items) {
    const settings = get(this, 'settings');
    return Promise.all([name || settings.findBySlug('dc'), items || this.findAll()]).then(
      ([name, items]) => {
        return this.findBySlug(name, items).catch(function() {
          const item = get(items, 'firstObject');
          settings.persist({ dc: get(item, 'Name') });
          return item;
        });
      }
    );
  },
  findAll: function() {
    return get(this, 'store')
      .findAll('dc')
      .then(function(items) {
        return items.sortBy('Name');
      });
  },
});
