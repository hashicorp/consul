/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

const MODEL_NAME = 'license';

const bucket = function (item, { dc, ns = 'default', partition = 'default' }) {
  return {
    ...item,
    Datacenter: dc,
    Namespace: typeof item.Namespace === 'undefined' ? ns : item.Namespace,
    Partition: typeof item.Partition === 'undefined' ? partition : item.Partition,
  };
};

const SECONDS = 1000;

export default class LicenseService extends RepositoryService {
  @dataSource('/:partition/:ns/:dc/license')
  async find({ partition, ns, dc }, { uri }, request) {
    return (
      await request`
      GET /v1/operator/license?${{ dc }}
      X-Request-ID: ${uri}
    `
    )((headers, body, cache) => ({
      meta: {
        version: 2,
        uri: uri,
        interval: 30 * SECONDS,
      },
      body: cache(
        bucket(body, { dc }),
        (uri) => uri`${MODEL_NAME}:///${partition}/${ns}/${dc}/license/${body.License.license_id}`
      ),
    }));
  }
}
