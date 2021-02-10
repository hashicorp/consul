import { get } from '@ember/object';
import RepositoryService from 'consul-ui/services/repository';

import tomographyFactory from 'consul-ui/utils/tomography';
import distance from 'consul-ui/utils/distance';
const tomography = tomographyFactory(distance);
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'coordinate';
export default class CoordinateService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  // Coordinates don't need nspaces so we have a custom method here
  // that doesn't accept nspaces
  @dataSource('/:ns/:dc/coordinates')
  findAllByDatacenter(params, configuration = {}) {
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), params);
  }

  @dataSource('/:ns/:dc/coordinates/for-node/:id')
  findAllByNode(params, configuration) {
    return this.findAllByDatacenter(params, configuration).then(function(coordinates) {
      let results = {};
      if (get(coordinates, 'length') > 1) {
        results = tomography(params.id, coordinates);
      }
      results.meta = get(coordinates, 'meta');
      return results;
    });
  }
}
