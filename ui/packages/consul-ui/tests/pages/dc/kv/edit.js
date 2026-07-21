/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, attribute, present, submitable, deletable, cancelable, kvs) {
  // The editor is a flyout over the list, not its own route. Scope to the
  // footer rather than the whole dialog: that is what disambiguates the KV's
  // own [data-test-delete] from the Lock Session section's identically
  // attributed "Invalidate Session" button in the flyout body.
  const footer = 'dialog .hds-flyout__footer';
  return {
    visit: visitable('/:dc/kv'),
    ...submitable({}, footer),
    ...cancelable({}, footer),
    ...deletable({}, footer),
    kvs: kvs(),
    kv: {
      Key: attribute('data-test-kv-key', 'dialog [data-test-kv-key]'),
    },
    session: {
      warning: present('dialog [data-test-session-warning]'),
      ID: attribute('data-test-session', 'dialog [data-test-session]'),
      // `delete` clicks the Invalidate trigger; `confirmDelete` clicks the
      // inline confirm that replaces it -- both carry [data-test-delete], only
      // one is present at a time.
      ...deletable({}, 'dialog [data-test-session]'),
    },
  };
}
