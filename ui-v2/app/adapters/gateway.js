import Adapter from './application';

export default Adapter.extend({
  requestForQueryRecord: function(request, { dc, ns, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/internal/ui/gateway-services-nodes/${id}?${{ dc }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  },
});
