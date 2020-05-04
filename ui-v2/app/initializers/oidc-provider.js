import Oauth2CodeProvider from 'torii/providers/oauth2-code';
const NAME = 'oidc-with-url';
const Provider = Oauth2CodeProvider.extend({
  name: NAME,
  buildUrl: function() {
    return this.baseUrl;
  },
  open: function(options) {
    const name = this.get('name'),
      url = this.buildUrl(),
      responseParams = ['state', 'code'],
      responseType = 'code';
    return this.get('popup')
      .open(url, responseParams, options)
      .then(function(authData) {
        // the same as the parent class but with an authorizationState added
        return {
          authorizationState: authData.state,
          authorizationCode: decodeURIComponent(authData[responseType]),
          provider: name,
        };
      });
  },
  close: function() {
    const popup = this.get('popup.remote') || {};
    if (typeof popup.close === 'function') {
      return popup.close();
    }
  },
});
export function initialize(application) {
  application.register(`torii-provider:${NAME}`, Provider);
}

export default {
  initialize,
};
