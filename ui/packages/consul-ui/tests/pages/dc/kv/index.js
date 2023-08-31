/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (visitable, creatable, kvs) {
  return creatable({
    visit: visitable(['/:dc/kv/:kv', '/:dc/kv'], (str) => str),
    kvs: kvs(),
  });
}
