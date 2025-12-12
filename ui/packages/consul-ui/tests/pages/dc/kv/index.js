/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, creatable, kvs) {
  return creatable({
    visit: visitable(['/:dc/kv/:kv', '/:dc/kv'], (str) => str),
    kvs: kvs(),
  });
}
