import Adapter from './application';
// TODO: Update to use this.formatDatacenter()
export default class CoordinateAdapter extends Adapter {
  requestForQuery(request, { dc, index, uri }) {
    return request`
      GET /v1/coordinate/nodes?${{ dc }}
      X-Request-ID: ${uri}

      ${{ index }}
    `;
  }
}
