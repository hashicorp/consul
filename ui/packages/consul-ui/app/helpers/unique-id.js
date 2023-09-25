/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class UniqueIdHelper extends Helper {
  @service('dom') dom;

  compute(params, hash) {
    return this.dom.guid({});
  }
}
