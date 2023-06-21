/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';

export function split([str = '', separator = ','], hash) {
  return str.split(separator);
}

export default helper(split);
