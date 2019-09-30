import Adapter from './application';

export default Adapter.extend({
  requestForFindAll: function(request) {
    return request`
      GET /v1/catalog/datacenters
    `;
  },
});
