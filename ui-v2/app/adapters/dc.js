import Adapter from './application';

export default Adapter.extend({
  requestForFindAll: function(request) {
    return request`
      GET /v1/catalog/datacenters
    `;
  },
  requestForQuery: function(request) {
    return request`
      GET /v1/catalog/datacenters
    `;
  },
});
