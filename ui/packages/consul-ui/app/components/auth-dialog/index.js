/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
    this.previousToken = null;
    this.args.onchange({ data: null, type: 'logout' });
  }
}
