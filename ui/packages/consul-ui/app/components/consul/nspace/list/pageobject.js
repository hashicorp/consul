/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, clickable, attribute, text, actions) => () => {
  const confirm = clickable("#confirm-modal [data-test-id='confirm-action']", {
    resetScope: true,
    testContainer: 'body', // modal is rendered in the body
  });
  return collection('.consul-nspace-list [data-test-tabular-row]', {
    nspace: clickable('a'),
    name: attribute('data-test-nspace', '[data-test-nspace]'),
    description: text('[data-test-description]'),
    actions: clickable('[data-test-actions-menu]'),
    edit: clickable('[data-test-edit]'),
    delete: clickable('[data-test-delete]'),
    confirmDelete: confirm,
  });
};
