/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import getStorage from 'consul-ui/utils/storage/local-storage';
const SCHEME = 'consul';
const storage = getStorage(SCHEME);
// promise aware assertion
export const ifNotBlocking = function (repo) {
  return repo.findBySlug('client').then(function (settings) {
    return typeof settings.blocking !== 'undefined' && !settings.blocking;
  });
};
export default class SettingsService extends Service {
  storage = storage;

  findAll(key) {
    return Promise.resolve(this.storage.all());
  }

  findBySlug(slug) {
    return Promise.resolve(this.storage.getValue(slug));
  }

  persist(obj) {
    const storage = this.storage;
    Object.keys(obj).forEach((item, i) => {
      storage.setValue(item, obj[item]);
    });
    return Promise.resolve(obj);
  }

  delete(obj) {
    // TODO: Loop through and delete the specified keys
    if (!Array.isArray(obj)) {
      obj = [obj];
    }
    const storage = this.storage;
    const item = obj.reduce(function (prev, item, i, arr) {
      storage.removeValue(item);
      return prev;
    }, {});
    return Promise.resolve(item);
  }
}
