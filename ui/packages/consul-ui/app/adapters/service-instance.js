import Adapter from './application';

// TODO: Update to use this.formatDatacenter()
export default class ServiceInstanceAdapter extends Adapter {
  requestForQuery(request, { dc, ns, partition, index, id, uri, peer }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }

    let options = {
      ns,
      partition,
      index,
    };

    if (peer) {
      options = {
        ...options,
        peer,
      };
    }

    return request`
      GET /v1/health/service/${id}?${{ dc, ['merge-central-config']: null }}
      X-Request-ID: ${uri}
      X-Range: ${id}

      ${options}
    `;
  }

  requestForQueryRecord() {
    // query and queryRecord both use the same endpoint
    // they are just serialized differently
    return this.requestForQuery(...arguments);
  }
}
