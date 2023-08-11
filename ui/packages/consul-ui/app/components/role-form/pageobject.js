/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
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
