import Adapter from './application';
import { inject as service } from '@ember/service';

export default class PermissionAdapter extends Adapter {
  @service('env') env;

  requestForAuthorize(request, { dc, ns, partition, resources = [], index }) {
    // the authorize endpoint is slightly different to all others in that it
    // ignores an ns parameter, but accepts a Namespace property on each
    // resource. Here we hide this difference from the rest of the app as
    // currently we never need to ask for permissions/resources for multiple
    // different namespaces in one call so here we use the ns param and add
    // this to the resources instead of passing through on the queryParameter
    //
    // ^ same goes for Partitions

    if (this.env.var('CONSUL_NSPACES_ENABLED')) {
      resources = resources.map(item => ({ ...item, Namespace: ns }));
    }
    if (this.env.var('CONSUL_PARTITIONS_ENABLED')) {
      resources = resources.map(item => ({ ...item, Partition: partition }));
    }
    return request`
      POST /v1/internal/acl/authorize?${{ dc }}

      ${resources}
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
