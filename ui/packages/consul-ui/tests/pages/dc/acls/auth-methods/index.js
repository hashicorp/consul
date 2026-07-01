/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, creatable, authMethods, clickable, collection) {
  return creatable({
    visit: visitable('/:dc/acls/auth-methods'),
    authMethods: authMethods(),
    sort: {
      selected: clickable('[data-test-sort-control] button', { at: 0 }),
      options: collection('[data-test-sort-option]', {
        resetScope: true,
        testContainer: 'html',
        button: clickable(),
      }),
    },
  });
}
