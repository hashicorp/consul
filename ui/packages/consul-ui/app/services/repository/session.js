/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'session';
export default class SessionService extends RepositoryService {
  @service('store')
  store;

  getModelName() {
    return modelName;
  }

  @dataSource('/:partition/:ns/:dc/sessions/for-node/:id')
  findByNode(params, configuration = {}) {
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), params);
  }

  // TODO: Why Key? Probably should be findBySlug like the others
  @dataSource('/:partition/:ns/:dc/sessions/for-key/:id')
  findByKey(params, configuration = {}) {
    return this.findBySlug(...arguments);
  }
}
