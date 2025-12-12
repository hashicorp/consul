/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, clickable, attribute, text, actions) => () => {
  return collection('.consul-policy-list [data-test-list-row]', {
    name: attribute('data-test-policy', '[data-test-policy]'),
    description: text('[data-test-description]'),
    policy: clickable('a', { at: 0 }),
    ...actions(['edit', 'delete']),
  });
};
