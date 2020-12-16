import Adapter from './application';
// TODO: Update to use this.formatDatacenter()
export default class ProxyAdapter extends Adapter {
  requestForQuery(request, { dc, ns, index, id, uri }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/catalog/connect/${id}?${{ dc }}
      X-Request-ID: ${uri}
      X-Range: ${id}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  }
}
