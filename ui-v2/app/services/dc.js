import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Service.extend({
  store: service('store'),
  settings: service('settings'),
  findBySlug: function(name, items) {
    if (name != null) {
      const item = items.findBy('Name', name);
      if (item) {
        // TODO: this does too much
        return get(this, 'settings')
          .persist({ dc: get(item, 'Name') })
          .then(function() {
            return { Name: get(item, 'Name') };
            // return item; // ?
          });
      }
    }
    return Promise.reject(items);
  },
  getActive: function(name, items) {
    const settings = get(this, 'settings');
    return Promise.all([name || settings.findBySlug('dc'), items || this.findAll()]).then(
      ([name, items]) => {
        return this.findBySlug(name, items).catch(function(items) {
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
