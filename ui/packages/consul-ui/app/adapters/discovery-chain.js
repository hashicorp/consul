import Adapter from './application';

// TODO: Update to use this.formatDatacenter()
export default class DiscoveryChainAdapter extends Adapter {
  requestForQueryRecord(request, { dc, ns, partition, index, id, uri }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/discovery-chain/${id}?${{ dc }}
      X-Request-ID: ${uri}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }
}
