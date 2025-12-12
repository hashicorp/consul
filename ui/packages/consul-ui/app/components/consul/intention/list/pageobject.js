/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, clickable, attribute, isPresent, deletable) =>
  (scope = '.consul-intention-list') => {
    const row = {
      source: attribute('data-test-intention-source', '[data-test-intention-source]'),
      destination: attribute(
        'data-test-intention-destination',
        '[data-test-intention-destination]'
      ),
      action: attribute('data-test-intention-action', '[data-test-intention-action]'),
      intention: clickable('a'),
      actions: clickable('label'),
      ...deletable(),
      delete: clickable('[data-test-delete] [role="menuitem"]'),
      confirmInlineDelete: clickable("#confirm-modal [data-test-id='confirm-action']", {
        resetScope: true,
        testContainer: 'body', // modal is rendered in the body
      }),
    };
    return {
      scope: scope,
      customResourceNotice: isPresent('.consul-intention-notice-custom-resource'),
      intentions: collection('[data-test-tabular-row]', row),
    };
  };
