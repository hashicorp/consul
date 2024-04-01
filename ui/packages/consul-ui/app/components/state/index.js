/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class State extends Component {
  @service('state') state;

  @tracked render = false;

  @action
  attributeChanged([state, matches, notMatches]) {
    if (typeof state === 'undefined') {
      return;
    }
    this.render =
      typeof matches !== 'undefined'
        ? this.state.matches(state, matches)
        : !this.state.matches(state, notMatches);
  }
}
