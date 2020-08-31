import Adapter from './application';

export default Adapter.extend({
  requestForQuery: function(request) {
    return request`
      GET /v1/catalog/datacenters
    `;
  },
});
