/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';
import { get } from '@ember/object';
const MANAGEMENT_ID = '00000000-0000-0000-0000-000000000001';
export function typeOf(params, hash) {
  const item = params[0];
  const template = get(item, 'template');
  switch (true) {
    case typeof template === 'undefined':
      return 'role';
    case template === 'service-identity':
      return 'policy-service-identity';
    case template === 'node-identity':
      return 'policy-node-identity';
    case get(item, 'ID') === MANAGEMENT_ID:
      return 'policy-management';
    default:
      return 'policy';
  }
}

export default helper(typeOf);
