/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Adapter from './application';

export default class HcpLinkAdapter extends Adapter {
  requestForQueryRecord(request, { dc, ns, partition, index, id }) {
    return request`
      GET /v2/link

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }
}
