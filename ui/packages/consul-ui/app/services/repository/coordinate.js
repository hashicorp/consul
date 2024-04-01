/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import RepositoryService from 'consul-ui/services/repository';

import dataSource from 'consul-ui/decorators/data-source';
import tomographyFactory from 'consul-ui/utils/tomography';
import distance from 'consul-ui/utils/distance';

const tomography = tomographyFactory(distance);

const modelName = 'coordinate';
export default class CoordinateService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  // Coordinates don't need nspaces so we have a custom method here
  // that doesn't accept nspaces
  @dataSource('/:partition/:ns/:dc/coordinates')
  async findAllByDatacenter(params, configuration = {}) {
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), params);
  }

  @dataSource('/:partition/:ns/:dc/coordinates/for-node/:id')
  async findAllByNode(params, configuration) {
    const coordinates = await this.findAllByDatacenter(params, configuration);

    let results = {};
    if (coordinates.length > 1) {
      results = tomography(params.id, coordinates);
    }
    results.meta = coordinates.meta;
    return results;
  }
}
