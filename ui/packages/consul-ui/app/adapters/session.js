/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Adapter from './application';

import { SLUG_KEY } from 'consul-ui/models/session';

// TODO: Update to use this.formatDatacenter()
export default class SessionAdapter extends Adapter {
  requestForQuery(request, { dc, ns, partition, index, id, uri }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/session/node/${id}?${{ dc }}
      X-Request-ID: ${uri}

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
      GET /v1/session/info/${id}?${{ dc }}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }

  requestForDeleteRecord(request, serialized, data) {
    const params = {
      dc: data.Datacenter,
      ns: data.Namespace,
      partition: data.Partition,
    };
    return request`
      PUT /v1/session/destroy/${data[SLUG_KEY]}?${params}
    `;
  }
}
