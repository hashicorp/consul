import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';
import { set } from '@ember/object';

const modelName = 'oidc-provider';
const OAUTH_PROVIDER_NAME = 'oidc-with-url';
export default RepositoryService.extend({
  manager: service('torii'),
  init: function() {
    this._super(...arguments);
    this.provider = getOwner(this).lookup(`torii-provider:${OAUTH_PROVIDER_NAME}`);
  },
  getModelName: function() {
    return modelName;
  },
  authorize: function(id, code, state, dc, nspace, configuration = {}) {
    const query = {
      id: id,
      code: code,
      state: state,
      dc: dc,
      ns: nspace,
    };
    return this.store.authorize(this.getModelName(), query);
  },
  logout: function(id, code, state, dc, nspace, configuration = {}) {
    // TODO: Temporarily call this secret, as we alreayd do that with
    // self in the `store` look to see whether we should just call it id like
    // the rest
    const query = {
      id: id,
    };
    return this.store.logout(this.getModelName(), query);
  },
  close: function() {
    this.manager.close(OAUTH_PROVIDER_NAME);
  },
  findCodeByURL: function(src) {
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
  },
});
