import Adapter from './application';
// TODO: Update to use this.formatDatacenter()
export default Adapter.extend({
  requestForQueryRecord: function(request, { dc, ns, index, id, uri }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/internal/ui/service-topology/${id}?${{ dc }}
      X-Request-ID: ${uri}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  },
});
