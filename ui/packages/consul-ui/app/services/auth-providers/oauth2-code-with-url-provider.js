/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import OAuth2CodeProvider from 'torii/providers/oauth2-code';
import { runInDebug } from '@ember/debug';

export default class OAuth2CodeWithURLProvider extends OAuth2CodeProvider {
  name = 'oidc-with-url';

  buildUrl() {
    return this.baseUrl;
  }

  open(options) {
    const name = this.name,
      url = this.buildUrl(),
      responseParams = ['state', 'code'],
      responseType = 'code';
    return this.popup.open(url, responseParams, options).then(function (authData) {
      // the same as the parent class but with an authorizationState added
      const creds = {
        authorizationState: authData.state,
        authorizationCode: decodeURIComponent(authData[responseType]),
        provider: name,
      };
      runInDebug((_) =>
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
