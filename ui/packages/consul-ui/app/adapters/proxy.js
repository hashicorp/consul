/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Adapter from './application';
// TODO: Update to use this.formatDatacenter()
export default class ProxyAdapter extends Adapter {
  requestForQuery(request, { dc, ns, partition, index, id, uri }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/catalog/connect/${id}?${{ dc, ['merge-central-config']: null }}
      X-Request-ID: ${uri}
      X-Range: ${id}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }
}
