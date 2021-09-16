import Error from '@ember/error';
import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'dc';
export default class DcService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  @dataSource('/:partition/:ns/:dc/datacenters')
  async findAll() {
    return super.findAll(...arguments);
  }

  @dataSource('/:partition/:ns/:dc/datacenter/:name')
  async findBySlug(params) {
    const items = this.store.peekAll('dc');
    const item = items.findBy('Name', params.name);
    if (typeof item === 'undefined') {
      // FIXME: HTTPError
      const e = new Error('Page not found');
      e.status = '404';
      // FIXME: EDError
      throw { errors: [e] };
    }
    return item;
  }
}
