import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import Error from '@ember/error';

const modelName = 'dc';
export default class DcService extends RepositoryService {
  @service('settings') settings;
  @service('env') env;

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
    const e = new Error('Page not found');
    e.status = '404';
    return Promise.reject({ errors: [e] });
  }

  getActive(name, items) {
    const settings = this.settings;
    return Promise.all([name || settings.findBySlug('dc'), items || this.findAll()]).then(
      ([name, items]) => {
        return this.findBySlug(name, items).catch(e => {
          const item =
            items.findBy('Name', this.env.var('CONSUL_DATACENTER_LOCAL')) ||
            get(items, 'firstObject');
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
