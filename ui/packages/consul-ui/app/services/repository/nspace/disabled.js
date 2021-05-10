import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/nspace';

const modelName = 'nspace';
const DEFAULT_NSPACE = 'default';
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
      Name: DEFAULT_NSPACE,
    };
  }

  authorize(dc, nspace) {
    return Promise.resolve([]);
  }
}
