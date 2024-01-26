/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

export default class HcpLinkService extends RepositoryService {
  /**
   * Data looks like
   * {
   *   "data": {
   *     "clientId": "5wZyAPvDFbgDdO3439m8tufwO9hElphu",
   *     "clientSecret": "SWX0XShcp3doc7RF8YCjJ-WATyeMAjFaf1eA0mnzlNHLF4IXbFz6xyjSZvHzAR_i",
   *     "resourceId": "organization/b4432207-bb9c-438e-a160-b98923efa979/project/4b09958c-fa91-43ab-8029-eb28d8cee9d4/hashicorp.consul.global-network-manager.cluster/test-from-api"
   *   },
   *   "generation": "01HMSDHXQTCQGD3Z68B3H58YFE",
   *   "id": {
   *     "name": "global",
   *     "tenancy": {
   *       "peerName": "local"
   *     },
   *     "type": {
   *       "group": "hcp",
   *       "groupVersion": "v2",
   *       "kind": "Link"
   *     },
   *     "uid": "01HMSDHXQTCQGD3Z68B10WBWHX"
   *   },
   *   "status": {
   *     "consul.io/hcp/link": {
   *       "conditions": [
   *         {
   *           "message": "Failed to link to HCP",
   *           "reason": "FAILED",
   *           "state": "STATE_FALSE",
   *           "type": "linked"
   *         }
   *       ],
   *       "observedGeneration": "01HMSDHXQTCQGD3Z68B3H58YFE",
   *       "updatedAt": "2024-01-22T20:24:57.141144170Z"
   *     }
   *   },
   *   "version": "57"
   * }
   */
  @dataSource('/:partition/:ns/:dc/hcp-link')
  async fetch({ partition, ns, dc }, { uri }, request) {
    let result;
    try {
      result = (
        await request`
      GET /api/hcp/v2/link/global
    `
      )((headers, body) => {
        const isLinked = (body.status['consul.io/hcp/link']['conditions'] || []).some(
          (condition) => condition.type === 'linked' && condition.state === 'STATE_TRUE'
        );
        const resourceId = body.data?.resourceId;

        return {
          meta: {
            version: 2,
            uri: uri,
          },
          body: {
            isLinked,
            resourceId,
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
