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
    // If the login response carried the provider's RP-Initiated logout URL
    // (present when the IdP advertises an end_session_endpoint), open it in a
    // new tab to terminate the IdP session too. This is a no-op when the field
    // isn't present (e.g. the IdP advertises no end_session_endpoint, or this
    // wasn't an SSO login).
    const idpLogoutURL = get(this, 'previousToken.IDPLogoutURL');
    if (typeof idpLogoutURL === 'string' && idpLogoutURL !== '') {
      // Only open well-formed http(s) URLs so an unexpected scheme (e.g.
      // javascript:) injected upstream can never be handed to window.open.
      let scheme;
      try {
        scheme = new URL(idpLogoutURL).protocol;
      } catch (e) {
        scheme = null;
      }
      if (scheme === 'http:' || scheme === 'https:') {
        window.open(idpLogoutURL, '_blank', 'noopener,noreferrer');
      }
    }
    this.previousToken = null;
    this.args.onchange({ data: null, type: 'logout' });
  }
}
