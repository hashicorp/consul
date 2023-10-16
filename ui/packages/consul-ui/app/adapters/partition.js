/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Adapter from './application';
import { SLUG_KEY } from 'consul-ui/models/partition';

// Blocking query support for partitions is currently disabled
export default class PartitionAdapter extends Adapter {
  async requestForQuery(request, { ns, dc, index }) {
    const respond = await request`
      GET /v1/partitions?${{ dc }}

      ${{ index }}
    `;
    await respond((headers, body) => delete headers['x-consul-index']);
    return respond;
  }
  // TODO: Not used until we do Partition CRUD
  async requestForQueryRecord(request, { ns, dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    const respond = await request`
      GET /v1/partition/${id}?${{ dc }}

      ${{ index }}
    `;
    await respond((headers, body) => delete headers['x-consul-index']);
    return respond;
  }

  async requestForCreateRecord(request, serialized, data) {
    return request`
      PUT /v1/partition/${data[SLUG_KEY]}?${{
      dc: data.Datacenter,
    }}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
      }}
    `;
  }

  async requestForUpdateRecord(request, serialized, data) {
    return request`
      PUT /v1/partition/${data[SLUG_KEY]}?${{
      dc: data.Datacenter,
    }}

      ${{
        Description: serialized.Description,
      }}
    `;
  }

  async requestForDeleteRecord(request, serialized, data) {
    return request`
      DELETE /v1/partition/${data[SLUG_KEY]}?${{
      dc: data.Datacenter,
    }}
    `;
  }
}
