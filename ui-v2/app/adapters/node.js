import Adapter from './application';
// TODO: Update to use this.formatDatacenter()

// Node and Namespaces are a little strange in that Nodes don't belong in a
// namespace whereas things that belong to a Node do (Health Checks and
// Service Instances). So even though Nodes themselves don't require a
// namespace filter, you sill needs to pass the namespace through to API
// requests in order to get the correct information for the things that belong
// to the node.

export default Adapter.extend({
  requestForQuery: function(request, { dc, ns, index, id }) {
    return request`
      GET /v1/internal/ui/nodes?${{ dc }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  },
  requestForQueryRecord: function(request, { dc, ns, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/internal/ui/node/${id}?${{ dc }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  },
  requestForQueryLeader: function(request, { dc }) {
    return request`
      GET /v1/status/leader?${{ dc }}
    `;
  },
  queryLeader: function(store, type, id, snapshot) {
    return this.request(
      function(adapter, request, serialized, unserialized) {
        return adapter.requestForQueryLeader(request, serialized, unserialized);
      },
      function(serializer, respond, serialized, unserialized) {
        return serializer.respondForQueryLeader(respond, serialized, unserialized);
      },
      snapshot,
      type.modelName
    );
  },
});
