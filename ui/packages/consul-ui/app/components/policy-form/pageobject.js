/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (submitable, cancelable, radiogroup, text) =>
  (scope = '[data-test-policy-form]') => {
    return {
      // this should probably be settable
      resetScope: true,
      scope: scope,
      get prefix() {
        return 'policy';
      },
      ...submitable(),
      ...cancelable(),
      ...radiogroup('template', ['', 'service-identity', 'node-identity'], 'policy'),
      rules: {
        error: text('[data-test-rules] strong'),
      },
    };
  };
