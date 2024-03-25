/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import { env } from 'consul-ui/env';

export default class EnvService extends Service {
  // deprecated
  // TODO: Remove this elsewhere in the app and use var instead
  env(key) {
    return this.var(key);
  }

  var(key) {
    return env(key);
  }
}
/**
 * Stub class that can be used in testing when we want to test
 * interactions with the EnvService. We can use `EnvStub.stubEnv` to setup
 * an Env-Service that returns certain values we need to execute our tests.
 *
 * Example:
 *
 * ```js
 * // some-test.js
 * test('testing interaction with Env-service', async function(assert) {
 *   this.owner.register('service:env', class Stub extends EnvStub {
 * .   stubEnv = {
 *       CONSUL_ACLS_ENABLED: true
 *     }
 *   })
 * })
 * ```
 */
export class EnvStub extends EnvService {
  var(key) {
    const { stubEnv } = this;

    const stubbed = stubEnv[key];

    if (stubbed) {
      return stubbed;
    } else {
      return super.var(...arguments);
    }
  }
}

/**
 * Helper function to allow stubbing out data that is accessed by the application
 * based on the Env-service. You will need to call this before the env-service gets
 * initialized because it overrides the env-service injection on the owner.
 *
 * Example:
 *
 * ```js
 * test('test something env related', async function(assert) {
 *   setupTestEnv(this.owner, {
 *     CONSUL_ACLS_ENABLED: true
 *   });
 *
 *   // ...
 * })
 * ```
 *
 * @param {*} owner - the owner of the test instance (usually `this.owner`)
 * @param {*} stubEnv - an object that holds the stubbed env-data
 */
export function setupTestEnv(owner, stubEnv) {
  owner.register(
    'service:env',
    class Stub extends EnvStub {
      stubEnv = stubEnv;
    }
  );
}
