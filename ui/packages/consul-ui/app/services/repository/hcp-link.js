/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

export default class HcpLinkService extends RepositoryService {
  @dataSource('/:partition/:ns/:dc/hcp-link')
  async fetch({ partition, ns, dc }, { uri }, request) {
    let result;
    try {
      result = (
        await request`
      GET /api/hcp/v2/link/global
    `
      )((headers, body) => {
        return {
          meta: {
            version: 2,
            uri: uri,
          },
          body: {
            isLinked: (body.status['consul.io/hcp/link']['conditions'] || []).some(
              (condition) => condition.type === 'linked' && condition.state === 'STATE_TRUE'
            ),
          },
          headers,
        };
      });
    } catch (e) {
      // set linked to false if the global link is not found
      if (e.statusCode === 404) {
        result = Promise.resolve({ isLinked: false });
      } else {
        result = Promise.resolve(null);
      }
    }
    return result;
  }
}
