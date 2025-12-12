/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

export default helper(function substr([str = '', start = 0, length], hash) {
  return str.substr(start, length);
});
