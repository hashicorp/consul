import Adapter from './application';

// TODO: Update to use this.formatDatacenter()

export default class NodeAdapter extends Adapter {
  requestForQuery(request, { dc, index, id, uri }) {
    return request`
      GET /v1/internal/ui/nodes?${{ dc }}
      X-Request-ID: ${uri}

      ${{ index }}
    `;
  }

  requestForQueryRecord(request, { dc, index, id, uri }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/internal/ui/node/${id}?${{ dc }}
      X-Request-ID: ${uri}

      ${{ index }}
    `;
  }

  requestForQueryLeader(request, { dc, uri }) {
    return request`
      GET /v1/status/leader?${{ dc }}
      X-Request-ID: ${uri}
      Refresh: 30
    `;
  }

  queryLeader(store, type, id, snapshot) {
    return this.rpc(
      function(adapter, request, serialized, unserialized) {
        return adapter.requestForQueryLeader(request, serialized, unserialized);
      },
      function(serializer, respond, serialized, unserialized) {
        return serializer.respondForQueryLeader(respond, serialized, unserialized);
      },
      snapshot,
      type.modelName
    );
  }
}
