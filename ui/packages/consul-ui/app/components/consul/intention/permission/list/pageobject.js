/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { collection, clickable } from 'ember-cli-page-object';

export default (scope = '.consul-intention-permission-list') => {
  return {
    scope: scope,
    intentionPermissions: collection('[data-test-list-row]', {
      edit: clickable('[data-test-edit-action]'),
      delete: clickable('[data-test-delete-action]'),
    }),
    confirmDelete: clickable('[data-test-confirm-action]'),
  };
};
