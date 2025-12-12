/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { getOwner } from '@ember/application';

export default class CachedHelper extends Helper {
  compute([model, params], hash) {
    const container = getOwner(this);
    return container.lookup(`service:repository/${model}`).cached(params);
  }
}
