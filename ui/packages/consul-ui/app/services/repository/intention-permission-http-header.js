/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import RepositoryService from 'consul-ui/services/repository';

const modelName = 'intention-permission-http-header';
export default class IntentionPermissionHttpHeaderService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  create(obj = {}) {
    return this.store.createFragment(this.getModelName(), obj);
  }

  persist(item) {
    return item.execute();
  }
}
