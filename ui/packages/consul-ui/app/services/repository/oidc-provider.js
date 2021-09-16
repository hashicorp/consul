import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import { getOwner } from '@ember/application';
import { set } from '@ember/object';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'oidc-provider';
const OAUTH_PROVIDER_NAME = 'oidc-with-url';
export default class OidcProviderService extends RepositoryService {
  @service('torii') manager;
  @service('settings') settings;

  init() {
    super.init(...arguments);
    this.provider = getOwner(this).lookup(`torii-provider:${OAUTH_PROVIDER_NAME}`);
  }

  getModelName() {
    return modelName;
  }

  @dataSource('/:partition/:ns/:dc/oidc/providers')
  async findAllByDatacenter() {
    return super.findAllByDatacenter(...arguments);
  }

  @dataSource('/:partition/:ns/:dc/oidc/provider/:id')
  async findBySlug(params) {
    // This addition is mainly due to ember-data book-keeping This is one of
    // the only places where Consul w/namespaces enabled doesn't return a
    // response with a Namespace property, but in order to keep ember-data
    // id's happy we need to fake one. Usually when we make a request to consul
    // with an empty `ns=` Consul will use the namespace that is assigned to
    // the token, and when we get the response we can pick that back off the
    // responses `Namespace` property. As we don't receive a `Namespace`
    // property here, we have to figure this out ourselves. Biut we also want
    // to make this completely invisible to 'the application engineer/a
    // template engineer'. This feels like the best place/way to do it as we
    // are already in a asynchronous method, and we avoid adding extra 'just
    // for us' parameters to the query object. There is a chance that as we
    // are discovering the tokens default namespace on the frontend and
    // assigning that to the ns query param, the token default namespace 'may'
    // have changed by the time the request hits the backend. As this is
    // extremely unlikely and in the scheme of things not a big deal, we
    // decided that doing this here is ok and avoids doing this in a more
    // complicated manner.
    const token = (await this.settings.findBySlug('token')) || {};
    return super.findBySlug({
      ns: params.ns || token.Namespace || 'default',
      dc: params.dc,
      id: params.id,
    });
  }

  @dataSource('/:partition/:ns/:dc/oidc/authorize/:id/:code/:state')
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
