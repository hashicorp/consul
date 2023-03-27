/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Adapter from './application';
import { SLUG_KEY } from 'consul-ui/models/nspace';

// namespaces aren't categorized by datacenter, therefore no dc
export default class NspaceAdapter extends Adapter {
  requestForQuery(request, { dc, partition, index, uri }) {
    return request`
      GET /v1/namespaces?${{ dc }}
      X-Request-ID: ${uri}

      ${{
        partition,
        index,
      }}
    `;
  }

  requestForQueryRecord(request, { dc, partition, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an name');
    }
    return request`
      GET /v1/namespace/${id}?${{ dc }}

      ${{
        partition,
        index,
      }}
    `;
  }

  requestForCreateRecord(request, serialized, data) {
    return request`
      PUT /v1/namespace/${data[SLUG_KEY]}?${{
      dc: data.Datacenter,
      partition: data.Partition,
    }}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        ACLs: {
          PolicyDefaults: serialized.ACLs.PolicyDefaults.map((item) => ({ ID: item.ID })),
          RoleDefaults: serialized.ACLs.RoleDefaults.map((item) => ({ ID: item.ID })),
        },
      }}
    `;
  }

  requestForUpdateRecord(request, serialized, data) {
    return request`
      PUT /v1/namespace/${data[SLUG_KEY]}?${{
      dc: data.Datacenter,
      partition: data.Partition,
    }}

      ${{
        Description: serialized.Description,
        ACLs: {
          PolicyDefaults: serialized.ACLs.PolicyDefaults.map((item) => ({ ID: item.ID })),
          RoleDefaults: serialized.ACLs.RoleDefaults.map((item) => ({ ID: item.ID })),
        },
      }}
    `;
  }

  requestForDeleteRecord(request, serialized, data) {
    return request`
      DELETE /v1/namespace/${data[SLUG_KEY]}?${{
      dc: data.Datacenter,
      partition: data.Partition,
    }}
    `;
  }
}
