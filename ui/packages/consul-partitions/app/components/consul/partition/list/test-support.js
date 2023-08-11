/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export const selectors = () => ({
  ['.consul-partition-list']: {
    row: {
      $: '[data-test-list-row]',
      partition: 'a',
      name: '[data-test-partition]',
      description: '[data-test-description]'
    }
  }
});
export const pageObject = (collection, clickable, attribute, text, actions) => () => {
  return collection('.consul-partition-list [data-test-list-row]', {
    partition: clickable('a'),
    name: attribute('data-test-partition', '[data-test-partition]'),
    description: text('[data-test-description]'),
    ...actions(['edit', 'delete']),
  });
};
