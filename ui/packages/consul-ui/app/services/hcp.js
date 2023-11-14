/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service, { inject as service } from '@ember/service';
import { runInDebug } from '@ember/debug';

/**
 * A service to encapsulate all logic that handles dealing with setting up consul
 * core correctly when started via HCP.
 */
export default class HCPService extends Service {
  @service('env') env;
  @service('repository/token') tokenRepo;
  @service('settings') settings;

  async updateTokenIfNecessary(secret) {
    if (secret) {
      const existing = await this.settings.findBySlug('token');

      if (secret && secret !== existing.SecretID) {
        try {
          const token = await this.tokenRepo.self({
            secret: secret,
            dc: this.env.var('CONSUL_DATACENTER_LOCAL'),
          });
          await this.settings.persist({
            token: {
              AccessorID: token.AccessorID,
              SecretID: token.SecretID,
              Namespace: token.Namespace,
              Partition: token.Partition,
            },
          });
        } catch (e) {
          runInDebug((_) => console.error(e));
        }
      }
    }
  }
}
