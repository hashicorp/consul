import Adapter from './application';
// TODO: Update to use this.formatDatacenter()
export default class CoordinateAdapter extends Adapter {
  requestForQuery(request, { dc, partition, index, uri }) {
    return request`
      GET /v1/coordinate/nodes?${{ dc }}
      X-Request-ID: ${uri}

      ${{
        partition,
        index,
      }}
    `;
  }
}
