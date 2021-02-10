import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import { getOwner } from '@ember/application';
import { set } from '@ember/object';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'oidc-provider';
const OAUTH_PROVIDER_NAME = 'oidc-with-url';
export default class OidcProviderService extends RepositoryService {
  @service('torii')
  manager;

  init() {
    super.init(...arguments);
    this.provider = getOwner(this).lookup(`torii-provider:${OAUTH_PROVIDER_NAME}`);
  }

  getModelName() {
    return modelName;
  }

  @dataSource('/:ns/:dc/oidc/providers')
  async findAllByDatacenter() {
    return super.findAllByDatacenter(...arguments);
  }

  @dataSource('/:ns/:dc/oidc/provider/:id')
  async findBySlug() {
    return super.findBySlug(...arguments);
  }

  @dataSource('/:ns/:dc/oidc/authorize/:id/:code/:state')
  authorize(params, configuration = {}) {
    return this.store.authorize(this.getModelName(), params);
  }

  logout(id, code, state, dc, nspace, configuration = {}) {
    // TODO: Temporarily call this secret, as we alreayd do that with
    // self in the `store` look to see whether we should just call it id like
    // the rest
    const query = {
      id: id,
    };
    return this.store.logout(this.getModelName(), query);
  }

  close() {
    this.manager.close(OAUTH_PROVIDER_NAME);
  }

  findCodeByURL(src) {
    // TODO: Maybe move this to the provider itself
    set(this.provider, 'baseUrl', src);
    return this.manager.open(OAUTH_PROVIDER_NAME, {}).catch(e => {
      let err;
      switch (true) {
        case e.message.startsWith('remote was closed'):
          err = new Error('Remote was closed');
          err.statusCode = 499;
          break;
        default:
          err = new Error(e.message);
          err.statusCode = 500;
      }
      this.store.adapterFor(this.getModelName()).error(err);
    });
  }
}
