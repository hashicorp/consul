/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
const MANAGEMENT_ID = '00000000-0000-0000-0000-000000000001';
const READ_ONLY_ID = '00000000-0000-0000-0000-000000000002';
export function typeOf(params, hash) {
  const item = params[0];
  const template = item?.template;
  switch (true) {
    case typeof template === 'undefined':
      return 'role';
    case template === 'service-identity':
      return 'policy-service-identity';
    case template === 'node-identity':
      return 'policy-node-identity';
    case item?.ID === MANAGEMENT_ID:
      return 'policy-management';
    case item?.ID === READ_ONLY_ID:
      return 'read-only';
    default:
      return 'policy';
  }
}

export default helper(typeOf);
