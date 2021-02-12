import Adapter from './application';

export default class PermissionAdapter extends Adapter {
  requestForAuthorize(request, { dc, ns, permissions, index }) {
    return request`
      POST /v1/internal/acl/authorize?${{ dc, ...this.formatNspace(ns), index }}

      ${permissions}
    `;
  }

  authorize(store, type, id, snapshot) {
    return this.rpc(
      function(adapter, request, serialized, unserialized) {
        return adapter.requestForAuthorize(request, serialized, unserialized);
      },
      function(serializer, respond, serialized, unserialized) {
        // Completely skip the serializer here
        return respond(function(headers, body) {
          return body;
        });
      },
      snapshot,
      type.modelName
    );
  }
}
