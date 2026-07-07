/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, clickable, attribute, deletable) => () => {
  return collection('.consul-kv-list [data-test-tabular-row]', {
    name: attribute('data-test-kv', '[data-test-kv]'),
    kv: clickable('a', { at: 0 }),
    toggle: clickable('[data-test-kv-toggle]'),
    ...deletable(),
    actions: clickable('[data-test-actions-menu]'),
    edit: clickable('[data-test-edit]'),
    delete: clickable('[data-test-delete]'),
    confirmInlineDelete: clickable("#confirm-modal [data-test-id='confirm-action']", {
      resetScope: true,
      testContainer: 'body', // modal is rendered in the body
    }),
  });
};
