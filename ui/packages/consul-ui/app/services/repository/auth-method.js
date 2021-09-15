import RepositoryService from 'consul-ui/services/repository';
import statusFactory from 'consul-ui/utils/acls-status';
import isValidServerErrorFactory from 'consul-ui/utils/http/acl/is-valid-server-error';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/auth-method';
import dataSource from 'consul-ui/decorators/data-source';

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

  @dataSource('/:partition/:ns/:dc/auth-methods')
  async findAllByDatacenter() {
    return super.findAllByDatacenter(...arguments);
  }

  @dataSource('/:partition/:ns/:dc/auth-method/:id')
  async findBySlug() {
    return super.findBySlug(...arguments);
  }

  status(obj) {
    return status(obj);
  }
}
