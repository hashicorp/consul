import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';

const modelName = 'session';
export default class SessionService extends RepositoryService {
  @service('store')
  store;

  getModelName() {
    return modelName;
  }

  findByNode(node, dc, nspace, configuration = {}) {
    const query = {
      id: node,
      dc: dc,
      ns: nspace,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), query);
  }

  // TODO: Why Key? Probably should be findBySlug like the others
  findByKey(slug, dc, nspace) {
    return this.findBySlug(...arguments);
  }
}
