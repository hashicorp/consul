import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'node';
export default class NodeService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  @dataSource('/:ns/:dc/leader')
  findLeader(params, configuration = {}) {
    if (typeof configuration.refresh !== 'undefined') {
      params.uri = configuration.uri;
    }
    return this.store.queryLeader(this.getModelName(), params);
  }
}
