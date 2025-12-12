/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (submitable, cancelable, policySelector) => () => {
  return {
    // this should probably be settable
    resetScope: true,
    scope: '[data-test-role-form]',
    get prefix() {
      return 'role';
    },
    ...submitable(),
    ...cancelable(),
    policies: policySelector('', '[data-test-create-policy]'),
  };
};
