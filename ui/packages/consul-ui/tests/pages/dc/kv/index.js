/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, creatable, kvs, submitable, deletable, cancelable) {
  // Footer-scoped, for the same reason as tests/pages/dc/kv/edit.js.
  const footer = 'dialog .hds-flyout__footer';
  return creatable({
    visit: visitable(['/:dc/kv/:kv', '/:dc/kv'], (str) => str),
    kvs: kvs(),
    ...submitable({}, footer),
    ...cancelable({}, footer),
    ...deletable({}, footer),
  });
}
