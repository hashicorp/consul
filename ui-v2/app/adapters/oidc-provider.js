import Adapter from './application';
import { inject as service } from '@ember/service';

import { env } from 'consul-ui/env';
import nonEmptySet from 'consul-ui/utils/non-empty-set';

let Namespace;
if (env('CONSUL_NSPACES_ENABLED')) {
  Namespace = nonEmptySet('Namespace');
} else {
  Namespace = () => ({});
}
export default Adapter.extend({
  env: service('env'),
  requestForQuery: function(request, { dc, ns, index }) {
    return request`
      GET /v1/internal/ui/oidc-auth-methods?${{ dc }}

      ${{
        index,
        ...this.formatNspace(ns),
      }}
    `;
  },
  requestForQueryRecord: function(request, { dc, ns, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      POST /v1/acl/oidc/auth-url?${{ dc }}
      Cache-Control: no-store

      ${{
        ...Namespace(ns),
        AuthMethod: id,
        RedirectURI: `${this.env.var('CONSUL_BASE_UI_URL')}/oidc/redirect.html`,
      }}
    `;
  },
  requestForAuthorize: function(request, { dc, ns, id, code, state }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    if (typeof code === 'undefined') {
      throw new Error('You must specify an code');
    }
    if (typeof state === 'undefined') {
      throw new Error('You must specify an state');
    }
    return request`
      POST /v1/acl/oidc/callback?${{ dc }}
      Cache-Control: no-store

      ${{
        ...Namespace(ns),
        AuthMethod: id,
        Code: code,
        State: state,
      }}
    `;
  },
  requestForLogout: function(request, { id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      POST /v1/acl/logout
      Cache-Control: no-store
      X-Consul-Token: ${id}
    `;
  },
  authorize: function(store, type, id, snapshot) {
    return this.request(
      function(adapter, request, serialized, unserialized) {
        return adapter.requestForAuthorize(request, serialized, unserialized);
      },
      function(serializer, respond, serialized, unserialized) {
        return serializer.respondForAuthorize(respond, serialized, unserialized);
      },
      snapshot,
      type.modelName
    );
  },
  logout: function(store, type, id, snapshot) {
    return this.request(
      function(adapter, request, serialized, unserialized) {
        return adapter.requestForLogout(request, serialized, unserialized);
      },
      function(serializer, respond, serialized, unserialized) {
        // its ok to return nothing here for the moment at least
        return {};
      },
      snapshot,
      type.modelName
    );
  },
});
