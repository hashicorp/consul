/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';

export function last([obj = ''], hash) {
  // TODO: Another candidate for a reusable type checking
  // util for helpers
  switch (true) {
    case typeof obj === 'string':
      return obj.substr(-1);
  }
}

export default helper(last);
