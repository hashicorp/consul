import Adapter from './application';
import { inject as service } from '@ember/service';

export default class ServiceAdapter extends Adapter {
  @service abilities;

  get peeringQuery() {
    const query = {};

    if (this.abilities.can('use peers')) {
      query['with-peers'] = true;
    }

    return query;
  }

  requestForQuery(request, { dc, ns, partition, index, gateway, uri }) {
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
        GET /v1/internal/ui/services?${{ dc }}
        X-Request-ID: ${uri}

        ${{
          ...this.peeringQuery,
          ns,
          partition,
          index,
        }}
    `;
    }
  }

  requestForQueryRecord(request, { dc, ns, partition, index, id, uri }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/health/service/${id}?${{ dc }}
      X-Request-ID: ${uri}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }
}
