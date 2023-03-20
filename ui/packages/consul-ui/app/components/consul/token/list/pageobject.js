/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default (collection, clickable, attribute, text, actions) => () => {
  return collection('.consul-token-list [data-test-list-row]', {
    id: attribute('data-test-token', '[data-test-token]'),
    description: text('[data-test-description]'),
    policy: text('[data-test-policy].policy', { multiple: true }),
    role: text('[data-test-policy].role', { multiple: true }),
    serviceIdentity: text('[data-test-policy].policy-service-identity', { multiple: true }),
    token: clickable('a'),
    ...actions(['edit', 'delete', 'use', 'logout', 'clone']),
  });
};
