import Adapter from './application';
// TODO: Update to use this.formatDatacenter()
export default Adapter.extend({
  requestForQuery: function(request, { dc, ns, index }) {
    return request`
      GET /v1/internal/ui/services?${{ dc }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  },
  requestForQueryRecord: function(request, { dc, ns, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/health/service/${id}?${{ dc }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  },
});
