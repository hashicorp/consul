/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export const selectors = () => ({
  ['.consul-partition-list-table']: {
    row: {
      $: '[data-test-tabular-row]',
      partition: 'a[data-test-partition]',
      name: '[data-test-partition]',
      description: '[data-test-description]',
    },
  },
});
export const pageObject = (collection, clickable, attribute, text, actions) => () => {
  return collection('.consul-partition-list-table [data-test-tabular-row]', {
    partition: clickable('a[data-test-partition]'),
    name: attribute('data-test-partition', '[data-test-partition]'),
    description: text('[data-test-description]'),
    ...actions(['edit', 'delete']),
  });
};
