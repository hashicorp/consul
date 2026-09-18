/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, attribute, present, submitable, deletable, clickable) {
  return {
    visit: visitable(['/:dc/kv/:kv/edit', '/:dc/kv/create'], function (str) {
      // this will encode the parts of the key path but means you can no longer
      // visit with path parts containing slashes
      return str.split('/').map(encodeURIComponent).join('/');
    }),
    ...submitable({}, 'main'),
    ...deletable(),
    // deleting a key is confirmed in a modal, which renders outside `main`
    confirmDelete: clickable("#confirm-modal [data-test-id='confirm-action']", {
      resetScope: true,
      testContainer: 'body',
    }),
    kv: {
      Key: attribute('data-test-kv-key', '[data-test-kv-key]'),
    },
    session: {
      warning: present('[data-test-session-warning]'),
      ID: attribute('data-test-session', '[data-test-session]'),
      ...deletable({}, '[data-test-session]'),
      // invalidating is confirmed in its own modal
      confirmDelete: clickable("#confirm-invalidate-modal [data-test-id='confirm-action']", {
        resetScope: true,
        testContainer: 'body',
      }),
    },
  };
}
