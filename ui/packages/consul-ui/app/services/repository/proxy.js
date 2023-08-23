import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY } from 'consul-ui/models/proxy';
import { get, set } from '@ember/object';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'proxy';
export default class ProxyService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  @dataSource('/:partition/:ns/:dc/proxies/for-service/:id')
  findAllBySlug(params, configuration = {}) {
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), params).then(items => {
      items.forEach(item => {
        // swap out the id for the services id
        // so we can then assign the proxy to it if it exists
        const id = JSON.parse(item.uid);
        id.pop();
        id.push(item.ServiceProxy.DestinationServiceID);
        const service = this.store.peekRecord('service-instance', JSON.stringify(id));
        if (service) {
          set(service, 'ProxyInstance', item);
        }
      });
      return items;
    });
  }

  @dataSource('/:partition/:ns/:dc/proxy-instance/:serviceId/:node/:id')
  async findInstanceBySlug(params, configuration) {
    const items = await this.findAllBySlug(params, configuration);

    let res = {};
    if (get(items, 'length') > 0) {
      let instance = items
        .filterBy('ServiceProxy.DestinationServiceID', params.serviceId)
        .findBy('NodeName', params.node);
      if (instance) {
        res = instance;
      } else {
        instance = items.findBy('ServiceProxy.DestinationServiceName', params.id);
        if (instance) {
          res = instance;
        }
      }
    }
    set(res, 'meta', get(items, 'meta'));
    return res;
  }
}
