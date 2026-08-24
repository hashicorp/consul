/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { get, action } from '@ember/object';
import chart from './chart.xstate';

export default class AuthDialog extends Component {
  @service('repository/oidc-provider') repo;

  constructor() {
    super(...arguments);
    this.chart = chart;
  }

  @action
  hasToken() {
    return typeof this.token.AccessorID !== 'undefined';
  }

  @action
  login() {
    let prev = get(this, 'previousToken.AccessorID');
    let current = get(this, 'token.AccessorID');
    if (prev === null) {
      prev = get(this, 'previousToken.SecretID');
    }
    if (current === null) {
      current = get(this, 'token.SecretID');
    }
    let type = 'authorize';
    if (typeof prev !== 'undefined' && prev !== current) {
      type = 'use';
    }
    this.args.onchange({ data: this.token, type: type });
  }

  @action
  logout() {
    if (typeof get(this, 'previousToken.AuthMethod') !== 'undefined') {
      // we are ok to fire and forget here
      this.repo.logout(get(this, 'previousToken.SecretID'));
    }
    // If the auth method has IdP (front-channel) logout enabled, the login
    // response carried the provider's RP-initiated logout URL. Open it in a
    // new tab to terminate the IdP session too, mirroring what `consul
    // logout` does on the CLI. This is a no-op when the field isn't present
    // (e.g. IdP logout disabled, or this wasn't an SSO login).
    const idpLogoutURL = get(this, 'previousToken.IDPLogoutURL');
    if (typeof idpLogoutURL === 'string' && idpLogoutURL !== '') {
      window.open(idpLogoutURL, '_blank', 'noopener,noreferrer');
    }
    this.previousToken = null;
    this.args.onchange({ data: null, type: 'logout' });
  }
}
