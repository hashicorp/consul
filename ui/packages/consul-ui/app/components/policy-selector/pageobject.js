/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (clickable, deletable, collection, alias, policyForm) =>
  (scope = '#policies', createSelector = '[data-test-policy-create]') => {
    return {
      scope: scope,
      create: clickable(createSelector),
      form: policyForm('#new-policy'),
      policies: alias('selectedOptions'),
      selectedOptions: collection(
        '[data-test-policies] [data-test-tabular-row]',
        deletable(
          {
            expand: clickable('label'),
          },
          '+ tr'
        )
      ),
    };
  };
