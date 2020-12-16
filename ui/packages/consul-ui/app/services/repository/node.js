import RepositoryService from 'consul-ui/services/repository';

const modelName = 'node';
export default class NodeService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  findLeader(dc, configuration = {}) {
    const query = {
      dc: dc,
    };
    if (typeof configuration.refresh !== 'undefined') {
      query.uri = configuration.uri;
    }
    return this.store.queryLeader(this.getModelName(), query);
  }
}
