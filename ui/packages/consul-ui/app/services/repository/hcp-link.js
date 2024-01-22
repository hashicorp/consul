/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'hcp-link';
export default class HcpLinkService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  @dataSource('/:partition/:ns/:dc/hcp-link')
  async findAll() {
    // TODO: check which method exactly is called here
    return super.findAll(...arguments);
  }
}
