import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/nspace';

const modelName = 'nspace';
export default class NspaceDisabledService extends RepositoryService {
  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  getSlugKey() {
    return SLUG_KEY;
  }

  getModelName() {
    return modelName;
  }

  findAll(configuration = {}) {
    return Promise.resolve([]);
  }

  getActive() {
    return {
      Name: 'default',
    };
  }

  authorize(dc, nspace) {
    return Promise.resolve([]);
  }
}
