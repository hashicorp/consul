/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

export function split([str = '', separator = ','], hash) {
  return str.split(separator);
}

export default helper(split);
