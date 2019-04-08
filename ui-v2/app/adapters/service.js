import Adapter from './application';
export default Adapter.extend({
  requestForQuery: function(request, { dc, index }) {
    return request`
      GET /v1/internal/ui/services?${{ dc }}

      ${{ index }}
    `;
  },
  requestForQueryRecord: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/health/service/${id}?${{ dc }}

      ${{ index }}
    `;
  },
});
