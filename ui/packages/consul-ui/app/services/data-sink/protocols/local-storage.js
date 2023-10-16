/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service, { inject as service } from '@ember/service';
import { setProperties } from '@ember/object';

export default class LocalStorageService extends Service {
  @service('settings')
  settings;

  prepare(sink, data, instance = {}) {
    if (data === null || data === '') {
      return instance;
    }
    return setProperties(instance, data);
  }

  persist(sink, instance) {
    const slug = sink.split(':').pop();
    const repo = this.settings;
    return repo.persist({
      [slug]: instance,
    });
  }

  remove(sink, instance) {
    const slug = sink.split(':').pop();
    const repo = this.settings;
    return repo.delete(slug);
  }
}
