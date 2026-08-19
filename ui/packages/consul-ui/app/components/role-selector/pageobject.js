/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { clickable } from 'ember-cli-page-object';

export default (clickableArg, deletable, collection, alias, roleForm) =>
  (scope = '#roles') => {
    return {
      scope: scope,
      create: clickableArg('[data-test-role-create]'),
      form: roleForm(),
      roles: alias('selectedOptions'),
      selectedOptions: collection('[data-test-roles] [data-test-tabular-row]', {
        actions: clickableArg('[data-test-role-actions]'),
        delete: clickableArg('[data-test-delete]'),
        confirmDelete: clickable("[data-test-id='confirm-action']", {
          resetScope: true,
          testContainer: 'body',
        }),
      }),
    };
  };
