import Adapter from './application';
// TODO: Update to use this.formatDatacenter()
export default Adapter.extend({
  requestForQuery: function(request, { dc, index, uri }) {
    return request`
      GET /v1/coordinate/nodes?${{ dc }}
      X-Request-ID: ${uri}

      ${{ index }}
    `;
  },
});
