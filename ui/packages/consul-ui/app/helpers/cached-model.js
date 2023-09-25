/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';
import { getOwner } from '@ember/application';

export default class CachedHelper extends Helper {
  compute([model, params], hash) {
    const container = getOwner(this);
    return container.lookup(`service:repository/${model}`).cached(params);
  }
}
