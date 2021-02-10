import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'service-instance';
export default class ServiceInstanceService extends RepositoryService {
  @service('repository/proxy') proxyRepo;
  getModelName() {
    return modelName;
  }

  @dataSource('/:ns/:dc/service-instances/for-service/:id')
  async findByService(params, configuration = {}) {
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), params);
  }

  @dataSource('/:ns/:dc/service-instance/:serviceId/:node/:id')
  async findBySlug(params, configuration = {}) {
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.store.queryRecord(this.getModelName(), params);
  }

  @dataSource('/:ns/:dc/proxy-service-instance/:serviceId/:node/:id')
  async findProxyBySlug(params, configuration = {}) {
    const instance = await this.findBySlug(...arguments);
    let proxy = this.store.peekRecord('proxy', instance.uid);
    // Currently, we call the proxy endpoint before this endpoint
    // therefore proxy is never undefined. If we ever call this endpoint
    // first we'll need to do something like the following
    // if(typeof proxy === 'undefined') {
    //   await proxyRepo.create({})
    // }

    // Copy over all the things to the ProxyServiceInstance
    ['Service', 'Node', 'meta'].forEach(prop => {
      set(proxy, prop, instance[prop]);
    });
    ['Checks'].forEach(prop => {
      // completely wipe out any previous values so we don't accumulate things
      // eternally
      proxy.set(prop, []);
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
