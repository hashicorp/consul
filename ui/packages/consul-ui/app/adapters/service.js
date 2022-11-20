import Adapter from './application';

export default class ServiceAdapter extends Adapter {
  requestForQuery(request, { dc, ns, partition, index, gateway, uri, peer }) {
    if (typeof gateway !== 'undefined') {
      return request`
        GET /v1/internal/ui/gateway-services-nodes/${gateway}?${{ dc }}
        X-Range: ${gateway}
        X-Request-ID: ${uri}

        ${{
          ns,
          partition,
          index,
        }}
      `;
    } else {
      return request`
        GET /v1/internal/ui/services?${{ dc, peer }}
        X-Request-ID: ${uri}

        ${{
          ns,
          partition,
          index,
          peer,
        }}
    `;
    }
  }

  requestForQueryRecord(request, { dc, ns, partition, index, id, uri }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/health/service/${id}?${{ dc, ['merge-central-config']: null }}
      X-Request-ID: ${uri}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }
}
