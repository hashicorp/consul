/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, clickable, attribute, deletable) => () => {
  return collection('.consul-kv-list [data-test-tabular-row]', {
    name: attribute('data-test-kv', '[data-test-kv]'),
    kv: clickable('a', { at: 0 }),
    actions: clickable('label', { at: 0 }),
    ...deletable(),
    delete: clickable('[data-test-delete] [role="menuitem"]', { at: 0 }),
    confirmInlineDelete: clickable("#confirm-modal [data-test-id='confirm-action']", {
      resetScope: true,
      testContainer: 'body', // modal is rendered in the body
    }),
  });
};
