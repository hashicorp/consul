import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/token';
import dataSource from 'consul-ui/decorators/data-source';

const MODEL_NAME = 'token';

export default class TokenService extends RepositoryService {
  @service('form') form;
  getModelName() {
    return MODEL_NAME;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  getSlugKey() {
    return SLUG_KEY;
  }

  @dataSource('/:partition/:ns/:dc/tokens')
  async findAll() {
    return super.findAll(...arguments);
  }

  @dataSource('/:partition/:ns/:dc/token/:id')
  async findBySlug(params) {
    let item;
    if (params.id === '') {
      item = await this.create({
        Datacenter: params.dc,
        Partition: params.partition,
        Namespace: params.ns,
      });
    } else {
      item = await super.findBySlug(...arguments);
    }
    return this.form
      .form(this.getModelName())
      .setData(item)
      .getData();
  }

  @dataSource('/:partition/:ns/:dc/token/self/:secret')
  self(params) {
    // This request does not need ns or partition passing through as its
    // inferred from the token itself.
    return this.store
      .self(this.getModelName(), {
        secret: params.secret,
        dc: params.dc,
      })
      .catch(e => {
        return Promise.reject(e);
      });
  }

  clone(item) {
    return this.store.clone(this.getModelName(), get(item, PRIMARY_KEY));
  }

  @dataSource('/:partition/:ns/:dc/tokens/for-policy/:policy')
  findByPolicy(params) {
    return this.findAll(...arguments);
  }

  @dataSource('/:partition/:ns/:dc/tokens/for-role/:role')
  findByRole() {
    return this.findAll(...arguments);
  }
}
