import Adapter from './application';

// TODO: Update to use this.formatDatacenter()
export default class ServiceInstanceAdapter extends Adapter {
  requestForQuery(request, { dc, ns, partition, index, id, uri }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/health/service/${id}?${{ dc }}
      X-Request-ID: ${uri}
      X-Range: ${id}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }

  requestForQueryRecord() {
    // query and queryRecord both use the same endpoint
    // they are just serialized differently
    return this.requestForQuery(...arguments);
  }
}
