/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class StateMatchesHelper extends Helper {
  @service('state') state;

  compute([state, values], hash) {
    return this.state.matches(state, values);
  }
}
