/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Adapter from './application';

// TODO: Update to use this.formatDatacenter()
export default class TopologyAdapter extends Adapter {
  requestForQueryRecord(request, { dc, ns, partition, kind, index, id, uri }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/internal/ui/service-topology/${id}?${{ dc, kind }}
      X-Request-ID: ${uri}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }
}
