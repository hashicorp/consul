import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';

const modelName = 'service-instance';
export default class ServiceInstanceService extends RepositoryService {
  @service('repository/proxy') proxyRepo;
  getModelName() {
    return modelName;
  }

  async findByService(slug, dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      ns: nspace,
      id: slug,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), query);
  }

  async findBySlug(serviceId, node, service, dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      ns: nspace,
      serviceId: serviceId,
      node: node,
      id: service,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.queryRecord(this.getModelName(), query);
  }

  async findProxyBySlug(serviceId, node, service, dc, nspace, configuration = {}) {
    const instance = await this.findBySlug(...arguments);
    let proxy = this.store.peekRecord('proxy', instance.uid);
    // Currently, we call the proxy endpoint before this endpoint
    // therefore proxy is never undefined. If we ever call this endpoint
    // first we'll need to do something like the following
    // if(typeof proxy === 'undefined') {
    //   await proxyRepo.create({})
    // }

    // Copy over all the things to the ProxyServiceInstance
    ['Service', 'Node'].forEach(prop => {
      set(proxy, prop, instance[prop]);
    });
    ['Checks'].forEach(prop => {
      instance[prop].forEach(item => {
        if (typeof item !== 'undefined') {
          proxy[prop].addFragment(item.copy());
        }
      });
    });
    // delete the ServiceInstance record as we now have a ProxyServiceInstance
    instance.unloadRecord();
    return proxy;
  }
}
