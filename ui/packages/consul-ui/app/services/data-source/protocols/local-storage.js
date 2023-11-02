/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service, { inject as service } from '@ember/service';
import { StorageEventSource } from 'consul-ui/utils/dom/event-source';

export default class LocalStorageService extends Service {
  @service('settings')
  repo;

  source(src, configuration) {
    const slug = src.split(':').pop();
    return new StorageEventSource(
      (configuration) => {
        return this.repo.findBySlug(slug);
      },
      {
        key: src,
        uri: configuration.uri,
      }
    );
  }
}
