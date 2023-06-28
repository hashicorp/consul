/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';

export default helper(function substr([str = '', start = 0, length], hash) {
  return str.substr(start, length);
});
