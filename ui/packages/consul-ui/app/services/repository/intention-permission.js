/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import RepositoryService from 'consul-ui/services/repository';
const modelName = 'intention-permission';
export default class IntentionPermissionService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  create(obj = {}) {
    // intention-permission and intention-permission-http
    // are currently treated as one and the same
    return this.store.createFragment(this.getModelName(), {
      ...obj,
      HTTP: this.store.createFragment('intention-permission-http', obj.HTTP || {}),
    });
  }

  persist(item) {
    return item.execute();
  }
}
