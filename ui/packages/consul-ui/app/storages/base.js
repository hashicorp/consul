/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default class Storage {
  constructor(key, storage) {
    this.key = key;
    this.storage = storage;

    this.state = this.initState(this.key, this.storage);
  }

  initState() {
    const { key, storage } = this;

    return storage.getItem(key);
  }
}
