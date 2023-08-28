/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

const _isLegacy = function (token) {
  // Empty Rules take priority over a Legacy: true
  // in order to make this decision
  const rules = get(token, 'Rules');
  if (rules != null) {
    return rules.trim() !== '';
  }
  const legacy = get(token, 'Legacy');
  if (typeof legacy !== 'undefined') {
    return legacy;
  }
  return false;
};
export function isLegacy(params, hash) {
  const token = params[0];
  // is array like (RecordManager isn't an array)
  if (typeof token.length !== 'undefined') {
    return token.find(function (item) {
      return _isLegacy(item);
    });
  }
  return _isLegacy(token);
}

export default helper(isLegacy);
