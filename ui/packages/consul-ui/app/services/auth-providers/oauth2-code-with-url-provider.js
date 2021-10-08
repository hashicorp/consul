import OAuth2CodeProvider from 'torii/providers/oauth2-code';
export default class OAuth2CodeWithURLProvider extends OAuth2CodeProvider {

  name = 'oidc-with-url';

  buildUrl() {
    return this.baseUrl;
  }

  open(options) {
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
  }

  close() {
    const popup = this.get('popup.remote') || {};
    if (typeof popup.close === 'function') {
      return popup.close();
    }
  }

}

