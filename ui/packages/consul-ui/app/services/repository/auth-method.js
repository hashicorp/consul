import RepositoryService from 'consul-ui/services/repository';
import statusFactory from 'consul-ui/utils/acls-status';
import isValidServerErrorFactory from 'consul-ui/utils/http/acl/is-valid-server-error';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/auth-method';

const isValidServerError = isValidServerErrorFactory();
const status = statusFactory(isValidServerError, Promise);
const MODEL_NAME = 'auth-method';

export default class AuthMethodService extends RepositoryService {
  getModelName() {
    return MODEL_NAME;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  getSlugKey() {
    return SLUG_KEY;
  }

  status(obj) {
    return status(obj);
  }
}
