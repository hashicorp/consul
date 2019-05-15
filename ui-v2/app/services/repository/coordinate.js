import { get } from '@ember/object';
import RepositoryService from 'consul-ui/services/repository';

import tomographyFactory from 'consul-ui/utils/tomography';
import distance from 'consul-ui/utils/distance';
const tomography = tomographyFactory(distance);

const modelName = 'coordinate';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  findAllByNode: function(node, dc, configuration) {
    return this.findAllByDatacenter(dc, configuration).then(function(coordinates) {
      let results = {};
      if (get(coordinates, 'length') > 1) {
        results = tomography(node, coordinates.map(item => get(item, 'data')));
      }
      results.meta = get(coordinates, 'meta');
      return results;
    });
  },
});
