/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

const ANONYMOUS_ID = '00000000-0000-0000-0000-000000000002';
export function isAnonymous(params, hash) {
  return params[0]?.AccessorID === ANONYMOUS_ID;
}
export default helper(isAnonymous);
