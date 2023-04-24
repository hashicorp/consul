import RepositoryService from 'consul-ui/services/repository';
import { set } from '@ember/object';
import { ACCESS_READ } from 'consul-ui/abilities/base';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'service-instance';
export default class ServiceInstanceService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  shouldReconcile(item, params) {
    return super.shouldReconcile(...arguments) && item.Service.Service === params.id;
  }

  @dataSource('/:partition/:ns/:dc/service-instances/for-service/:id/:peer')
  async findByService(params, configuration = {}) {
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.authorizeBySlug(
      async resources => {
        const instances = await this.query(params);
        set(instances, 'firstObject.Service.Resources', resources);
        return instances;
      },
      ACCESS_READ,
      params
    );
  }

  @dataSource('/:partition/:ns/:dc/service-instance/:serviceId/:node/:id/:peer')
  async findBySlug(params, configuration = {}) {
    return super.findBySlug(...arguments);
  }
}
