/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, clickable, attribute, text, actions) => () => {
  const confirm = clickable("#confirm-modal [data-test-id='confirm-action']", {
    resetScope: true,
    testContainer: 'body', // modal is rendered in the body
  });
  return collection('.consul-role-list [data-test-tabular-row]', {
    role: clickable('a', { at: 0 }),
    name: attribute('data-test-role', '[data-test-role]'),
    description: text('[data-test-description]'),
    policy: text('[data-test-policy][data-type="policy"]', {
      multiple: true,
    }),
    serviceIdentity: text('[data-test-policy][data-type="policy-service-identity"]', {
      multiple: true,
    }),
    actions: clickable('[data-test-actions-menu]'),
    edit: clickable('[data-test-edit]'),
    delete: clickable('[data-test-delete]'),
    confirmDelete: confirm,
  });
};
