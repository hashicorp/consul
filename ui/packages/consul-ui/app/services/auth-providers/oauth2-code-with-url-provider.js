/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import OAuth2CodeProvider from 'torii/providers/oauth2-code';
import { runInDebug } from '@ember/debug';

export default class OAuth2CodeWithURLProvider extends OAuth2CodeProvider {
  name = 'oidc-with-url';

  buildUrl() {
    return this._lastBaseUrl || this.baseUrl;
  }

  open(options = {}) {
    if (options.baseUrl) {
      this._lastBaseUrl = options.baseUrl;
    }
    const name = this.name,
      url = options.baseUrl || this.buildUrl(),
      responseParams = ['state', 'code'],
      responseType = 'code';
    return this.popup.open(url, responseParams, options).then((authData) => {
      const creds = {
        authorizationState: authData.state,
        authorizationCode: decodeURIComponent(authData[responseType]),
        provider: name,
      };
      runInDebug(() =>
        console.info('Retrieved the following creds from the OAuth Provider', creds)
      );
      return creds;
    });
  }

  close() {
    const popup = this.get('popup.remote') || {};
    if (typeof popup.close === 'function') {
      return popup.close();
    }
  }
}
