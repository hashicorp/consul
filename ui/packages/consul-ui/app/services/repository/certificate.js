/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';
import { inject as service } from '@ember/service';

const modelName = 'certificate';

export default class CertificateService extends RepositoryService {
  @service store;

  getModelName() {
    return modelName;
  }

  @dataSource('/:partition/:ns/:dc/certificates')
  async fetchAll({ partition, ns, dc }, { uri }, request) {
    return (
      await request`
      GET /v1/internal/ui/certificates-expiry-days/
      X-Request-ID: ${uri}
    `
    )((headers, body, cache) => {
      let idx = 0;
      for (const cert of body) {
        this.store.push({
          data: { id: idx++, type: 'certificate', attributes: cert },
        });
      }
      return {
        meta: {
          version: 2,
          uri: uri,
        },
        body: body,
      };
    });
  }

  @dataSource('/:partition/:ns/:dc/certificates-cache')
  async find(params) {
    const items = this.store.peekAll('certificate').toArray();
    return items;
  }
}
