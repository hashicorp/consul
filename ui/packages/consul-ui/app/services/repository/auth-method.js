/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/auth-method';
import dataSource from 'consul-ui/decorators/data-source';

const MODEL_NAME = 'auth-method';

export default class AuthMethodService extends RepositoryService {
  getModelName() {
    return MODEL_NAME;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  getSlugKey() {
    return SLUG_KEY;
  }

  @dataSource('/:partition/:ns/:dc/auth-methods')
  async findAllByDatacenter() {
    return super.findAllByDatacenter(...arguments);
  }

  @dataSource('/:partition/:ns/:dc/auth-method/:id')
  async findBySlug() {
    return super.findBySlug(...arguments);
  }
}
