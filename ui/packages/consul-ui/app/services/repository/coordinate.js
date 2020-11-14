import { get } from '@ember/object';
import RepositoryService from 'consul-ui/services/repository';

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
  findAllByDatacenter(dc, configuration = {}) {
    const query = {
      dc: dc,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), query);
  }

  findAllByNode(node, dc, configuration) {
    return this.findAllByDatacenter(dc, configuration).then(function(coordinates) {
      let results = {};
      if (get(coordinates, 'length') > 1) {
        results = tomography(
          node,
          coordinates.map(item => get(item, 'data'))
        );
      }
      results.meta = get(coordinates, 'meta');
      return results;
    });
  }
}
