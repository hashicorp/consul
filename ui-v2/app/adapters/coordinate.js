import Adapter from './http';
export default Adapter.extend({
  requestForQuery: function(request, { dc, index }) {
    return request`
      GET /v1/coordinate/nodes?${{ dc, index }}
    `;
  },
});
