import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import Error from '@ember/error';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'dc';
export default class DcService extends RepositoryService {
  @service('settings') settings;
  @service('env') env;

  getModelName() {
    return modelName;
  }

  @dataSource('/:ns/:dc/datacenters')
  async findAll(params, configuration = {}) {
    return this.store.query(this.getModelName(), {});
  }

  async findBySlug(name) {
    const items = this.store.peekAll('dc');
    if (name != null) {
      const item = await items.findBy('Name', name);
      if (typeof item !== 'undefined') {
        return item;
      }
    }
    const e = new Error('Page not found');
    e.status = '404';
    return Promise.reject({ errors: [e] });
  }

  async getActive(name, items) {
    return Promise.all([name, items || this.findAll()]).then(([name, items]) => {
      return this.findBySlug(name, items).catch(async e => {
        return (
          items.findBy('Name', this.env.var('CONSUL_DATACENTER_LOCAL')) || get(items, 'firstObject')
        );
      });
    });
  }

  async clearActive() {
    return this.settings.delete('dc');
  }
}
