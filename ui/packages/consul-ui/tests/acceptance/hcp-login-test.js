/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupTestEnv } from 'consul-ui/services/env';
import TokenRepo from 'consul-ui/services/repository/token';
import SettingsService from 'consul-ui/services/settings';

const TOKEN_SET_BY_HCP = 'token-set-by-hcp';

module('Acceptance | hcp login', function (hooks) {
  setupApplicationTest(hooks);

  module('with `CONSUL_HTTP_TOKEN` not set', function (hooks) {
    hooks.beforeEach(function () {
      setupTestEnv(this.owner, {
        CONSUL_ACLS_ENABLED: true,
      });
    });

    test('we do not call the token endpoint', async function (assert) {
      this.owner.register(
        'service:repository/token',
        class extends TokenRepo {
          self() {
            assert.step('token');

            return super.self(...arguments);
          }
        }
      );

      await visit('/');

      assert.verifySteps([], 'we do not try to fetch new token');
    });
  });

  module('with `CONSUL_HTTP_TOKEN` set', function (hooks) {
    hooks.beforeEach(function () {
      setupTestEnv(this.owner, {
        CONSUL_ACLS_ENABLED: true,
        CONSUL_HTTP_TOKEN: TOKEN_SET_BY_HCP,
      });
    });

    test('when no token was persisted to settings', async function (assert) {
      assert.expect(3);

      // stub out the settings service to not access local-storage directly
      this.owner.register(
        'service:settings',
        class extends SettingsService {
          async findBySlug(slug) {
            // make sure we don't find anything
            if (slug === 'token') {
              // we return an empty string if nothing is found
              return Promise.resolve('');
            } else {
              return super.findBySlug(...arguments);
            }
          }
        }
      );

      // There's no way to hook into the api handlers like with mirage
      // so we need to stub the repo methods
      this.owner.register(
        'service:repository/token',
        class extends TokenRepo {
          self(params) {
            const { secret } = params;

            assert.equal(
              secret,
              TOKEN_SET_BY_HCP,
              'we try to request token based on what HCP set for us'
            );

            assert.step('token');

            return super.self(...arguments);
          }
        }
      );

      await visit('/');

      assert.verifySteps(['token'], 'we try to call token endpoint to fetch new token');
    });

    test('when we already persisted a token to settings and it is different to the secret HCP set for us', async function (assert) {
      assert.expect(3);

      this.owner.register(
        'service:settings',
        class extends SettingsService {
          async findBySlug(slug) {
            if (slug === 'token') {
              return Promise.resolve({
                AccessorID: 'accessor',
                SecretID: 'secret',
                Namespace: 'default',
                Partition: 'default',
              });
            } else {
              return super.findBySlug(...arguments);
            }
          }
        }
      );

      this.owner.register(
        'service:repository/token',
        class extends TokenRepo {
          self(params) {
            const { secret } = params;

            assert.equal(
              secret,
              TOKEN_SET_BY_HCP,
              'we try to request token based on what HCP set for us'
            );

            assert.step('token');

            return super.self(...arguments);
          }
        }
      );

      await visit('/');

      assert.verifySteps(['token'], 'we call token endpoint to fetch new token');
    });

    test('when we already persisted a token to settings, but it is the same secret as HCP set for us', async function (assert) {
      assert.expect(1);

      this.owner.register(
        'service:settings',
        class extends SettingsService {
          async findBySlug(slug) {
            if (slug === 'token') {
              return Promise.resolve({
                AccessorID: 'accessor',
                SecretID: TOKEN_SET_BY_HCP,
                Namespace: 'default',
                Partition: 'default',
              });
            } else {
              return super.findBySlug(...arguments);
            }
          }
        }
      );

      this.owner.register(
        'service:repository/token',
        class extends TokenRepo {
          self() {
            assert.step('token');

            return super.self(...arguments);
          }
        }
      );

      await visit('/');

      assert.verifySteps([], 'we do not try to fetch new token');
    });
  });
});
