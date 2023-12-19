/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';
import { isEmpty } from '@ember/utils';
import { A as emberArray } from '@ember/array';

export function uniqBy([byPath, array]) {
  if (isEmpty(byPath)) {
    return [];
  }

  return emberArray(array).uniqBy(byPath);
}

export default helper(uniqBy);
