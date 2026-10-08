/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, clickable, attribute) => () => {
  const confirm = clickable("#confirm-modal [data-test-id='confirm-action']", {
    resetScope: true,
    testContainer: 'body', // modal is rendered in the body
  });
  return collection('.consul-peer-list [data-test-tabular-row]', {
    name: attribute('data-test-peer', '[data-test-peer]'),
    actions: clickable('[data-test-actions-menu]'),
    regenerate: clickable('[data-test-regenerate]'),
    view: clickable('[data-test-view]'),
    delete: clickable('[data-test-delete]'),
    confirmDelete: confirm,
  });
};
