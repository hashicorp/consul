import RepositoryService from 'consul-ui/services/repository';
const modelName = 'service';
export default class _RepositoryService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  findGatewayBySlug(slug, dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      ns: nspace,
      gateway: slug,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), query);
  }
}
