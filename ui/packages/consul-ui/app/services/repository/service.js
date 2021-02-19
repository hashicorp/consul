import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'service';
export default class ServiceService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  @dataSource('/:ns/:dc/services')
  async findAllByDatacenter() {
    return super.findAllByDatacenter(...arguments);
  }

  @dataSource('/:ns/:dc/gateways/for-service/:gateway')
  findGatewayBySlug(params, configuration = {}) {
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), params);
  }
}
