/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, clickable, attribute, text) => () => {
  const confirm = clickable("#confirm-modal [data-test-id='confirm-action']", {
    resetScope: true,
    testContainer: 'body', // modal is rendered in the body
  });
  return collection('.consul-token-list [data-test-tabular-row]', {
    id: attribute('data-test-token', '[data-test-token]'),
    name: text('[data-test-name]'),
    description: text('[data-test-description]'),
    policy: text('[data-test-policy][data-type="policy"]', {
      multiple: true,
    }),
    role: text('[data-test-policy][data-type="role"]', {
      multiple: true,
    }),
    serviceIdentity: text('[data-test-policy][data-type="policy-service-identity"]', {
      multiple: true,
    }),
    token: clickable('a', { at: 0 }),
    actions: clickable('[data-test-actions-menu]'),
    edit: clickable('[data-test-edit]'),
    clone: clickable('[data-test-clone]'),
    use: clickable('[data-test-use]'),
    logout: clickable('[data-test-logout]'),
    delete: clickable('[data-test-delete]'),
    confirmUse: confirm,
    confirmLogout: confirm,
    confirmDelete: confirm,
  });
};
