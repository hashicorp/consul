import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import Error from '@ember/error';

const modelName = 'dc';
export default RepositoryService.extend({
  settings: service('settings'),
  getModelName: function() {
    return modelName;
  },
  findAll: function() {
    return this.store.query(this.getModelName(), {}).then(function(items) {
      // TODO: Move to view/template
      return items.sortBy('Name');
    });
  },
  findBySlug: function(name, items) {
    if (name != null) {
      const item = items.findBy('Name', name);
      if (item) {
        return this.settings.persist({ dc: get(item, 'Name') }).then(function() {
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
    const settings = this.settings;
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
  clearActive: function() {
    return this.settings.delete('dc');
  },
});
