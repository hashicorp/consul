/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default (submitable, cancelable, policySelector) => () => {
  return {
    // this should probably be settable
    resetScope: true,
    scope: '[data-test-role-form]',
    prefix: 'role',
    ...submitable(),
    ...cancelable(),
    policies: policySelector('', '[data-test-create-policy]'),
  };
};
