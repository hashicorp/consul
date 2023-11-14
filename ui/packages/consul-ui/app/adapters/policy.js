/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Adapter from './application';
import { SLUG_KEY } from 'consul-ui/models/policy';

// TODO: Update to use this.formatDatacenter()
export default class PolicyAdapter extends Adapter {
  requestForQuery(request, { dc, ns, partition, index, id }) {
    return request`
      GET /v1/acl/policies?${{ dc }}

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
      GET /v1/acl/policy/${id}?${{ dc }}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }

  requestForCreateRecord(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data.Datacenter),
      ns: data.Namespace,
      partition: data.Partition,
    };
    return request`
      PUT /v1/acl/policy?${params}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        Rules: serialized.Rules,
        Datacenters: serialized.Datacenters,
      }}
    `;
  }

  requestForUpdateRecord(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data.Datacenter),
      ns: data.Namespace,
      partition: data.Partition,
    };
    return request`
      PUT /v1/acl/policy/${data[SLUG_KEY]}?${params}

      ${{
        Name: serialized.Name,
        Description: serialized.Description,
        Rules: serialized.Rules,
        Datacenters: serialized.Datacenters,
      }}
    `;
  }

  requestForDeleteRecord(request, serialized, data) {
    const params = {
      ...this.formatDatacenter(data.Datacenter),
      ns: data.Namespace,
      partition: data.Partition,
    };
    return request`
      DELETE /v1/acl/policy/${data[SLUG_KEY]}?${params}
    `;
  }
}
