/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, clickable, attribute, deletable) => () => {
  return collection('.consul-kv-list [data-test-tabular-row]', {
    name: attribute('data-test-kv', '[data-test-kv]'),
    kv: clickable('a'),
    actions: clickable('label'),
    ...deletable(),
    delete: clickable('[data-test-delete] [role="menuitem"]'),
    confirmInlineDelete: clickable("#confirm-modal [data-test-id='confirm-action']", {
      resetScope: true,
      testContainer: 'body', // modal is rendered in the body
    }),
  });
};
