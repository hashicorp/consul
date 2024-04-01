/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Adapter from './application';

export default class AuthMethodAdapter extends Adapter {
  requestForQuery(request, { dc, ns, partition, index, id }) {
    return request`
      GET /v1/acl/auth-methods?${{ dc }}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }

  requestForQueryRecord(request, { dc, ns, partition, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/acl/auth-method/${id}?${{ dc }}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }
}
