/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Adapter from './application';
import { inject as service } from '@ember/service';

export default class OidcProviderAdapter extends Adapter {
  @service('env') env;

  requestForQuery(request, { dc, ns, partition, index, uri }) {
    return request`
      GET /v1/internal/ui/oidc-auth-methods?${{ dc }}
      X-Request-ID: ${uri}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }

  requestForQueryRecord(request, { dc, ns, partition, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      POST /v1/acl/oidc/auth-url?${{ dc, ns, partition }}
      Cache-Control: no-store

      ${{
        AuthMethod: id,
        RedirectURI: `${this.env.var('CONSUL_BASE_UI_URL')}/oidc/callback`,
      }}
    `;
  }

  requestForAuthorize(request, { dc, ns, partition, id, code, state }) {
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
      POST /v1/acl/oidc/callback?${{ dc, ns, partition }}
      Cache-Control: no-store

      ${{
        AuthMethod: id,
        Code: code,
        State: state,
      }}
    `;
  }

  requestForLogout(request, { id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      POST /v1/acl/logout
      Cache-Control: no-store
      X-Consul-Token: ${id}
    `;
  }

  authorize(store, type, id, snapshot) {
    return this.rpc(
      function (adapter, request, serialized, unserialized) {
        return adapter.requestForAuthorize(request, serialized, unserialized);
      },
      function (serializer, respond, serialized, unserialized) {
        return serializer.respondForAuthorize(respond, serialized, unserialized);
      },
      snapshot,
      type.modelName
    );
  }

  logout(store, type, id, snapshot) {
    return this.rpc(
      function (adapter, request, serialized, unserialized) {
        return adapter.requestForLogout(request, serialized, unserialized);
      },
      function (serializer, respond, serialized, unserialized) {
        // its ok to return nothing here for the moment at least
        return {};
      },
      snapshot,
      type.modelName
    );
  }
}
