import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import Error from '@ember/error';

const modelName = 'dc';
export default class DcService extends RepositoryService {
  @service('settings')
  settings;

  getModelName() {
    return modelName;
  }

  findAll() {
    return this.store.query(this.getModelName(), {});
  }

  findBySlug(name, items) {
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
  }

  getActive(name, items) {
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
  }

  clearActive() {
    return this.settings.delete('dc');
  }
}
