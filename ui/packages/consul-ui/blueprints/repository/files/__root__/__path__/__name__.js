/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

const MODEL_NAME = '<%= dasherizedModuleName %>';
const PRIMARY_KEY = 'uid';
const SLUG_KEY = 'ID';
export default class <%= classifiedModuleName %>Repository extends RepositoryService {
  getModelName() {
    return MODEL_NAME;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  getSlugKey() {
    return SLUG_KEY;
  }

  @dataSource('/:ns/:dc/<%= dasherizedModuleName %>')
  async findAllByDatacenter() {
    return super.findAllByDatacenter(...arguments);
  }

  @dataSource('/:ns/:dc/<%= dasherizedModuleName %>/:id')
  async findBySlug() {
    return super.findBySlug(...arguments);
  }
}
