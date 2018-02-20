import Service from '@ember/service';

import get from 'consul-ui/utils/request/get';
import map from 'consul-ui/utils/map';
import Entity from 'consul-ui/models/dc/node';

export default Service.extend({
  findAllByDatacenter: function(dc) {
    return get('/v1/internal/ui/nodes', dc).then(map(Entity));
  },
  findAllCoordinatesByDatacenter: function(dc) {
    return get('/v1/coordinate/nodes', dc);
  },
  findBySlug: function(slug, dc) {
    // maintain consistency with map([])
    return get('/v1/internal/ui/node/' + slug, dc).then(map([Entity])[0]);
  },
});
