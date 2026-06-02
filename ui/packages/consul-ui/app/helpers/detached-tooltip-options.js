/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

export default class DetachedTooltipOptionsHelper extends Helper {
  compute(_params, hash) {
    if (typeof document === 'undefined') {
      return hash;
    }

    return {
      ...hash,
      appendTo: document.body,
    };
  }
}
