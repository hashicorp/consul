/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { clickable } from 'ember-cli-page-object';

export default (clickableArg, deletable, collection, alias, policyForm) =>
  (scope = '#policies', createSelector = '[data-test-policy-create]') => {
    const confirmDelete = clickable("[data-test-id='confirm-action']", {
      resetScope: true,
      testContainer: 'body',
    });
    return {
      scope: scope,
      create: clickableArg(createSelector),
      form: policyForm('#new-policy'),
      policies: alias('selectedOptions'),
      selectedOptions: collection('[data-test-policies] .hds-accordion-item', {
        expand: clickableArg('button.hds-accordion-item__button'),
        delete: clickableArg('[data-test-delete]'),
        confirmDelete: confirmDelete,
      }),
    };
  };
