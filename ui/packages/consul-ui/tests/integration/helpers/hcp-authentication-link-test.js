/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { setupRenderingTest } from 'ember-qunit';
import { HCP_PREFIX } from 'consul-ui/helpers/hcp-authentication-link';
import { EnvStub } from 'consul-ui/services/env';

const clusterName = 'hello';
const clusterVersion = '1.18.0';
const accessMode = 'CONSUL_ACCESS_LEVEL_GLOBAL_READ_WRITE';
module('Integration | Helper | hcp-authentication-link', function (hooks) {
  setupRenderingTest(hooks);
  hooks.beforeEach(function () {
    this.owner.register(
      'service:env',
      class Stub extends EnvStub {
        stubEnv = {
          CONSUL_VERSION: clusterVersion,
        };
      }
    );
  });
  test('it makes a URL out of a real resourceId', async function (assert) {
    this.dcName = clusterName;

    await render(hbs`{{hcp-authentication-link dcName}}`);

    assert.equal(
      this.element.textContent.trim(),
      `${HCP_PREFIX}?cluster_name=${clusterName}&cluster_version=${clusterVersion}`
    );
  });

  test('it returns correct link without dc name', async function (assert) {
    this.dcName = null;

    await render(hbs`{{hcp-authentication-link dcName}}`);
    assert.equal(
      this.element.textContent.trim(),
      `${HCP_PREFIX}?cluster_version=${clusterVersion}`
    );
  });

  test('it makes a URL out of a dc name and accessLevel, if passed', async function (assert) {
    this.dcName = clusterName;
    this.accessMode = accessMode;

    await render(hbs`{{hcp-authentication-link dcName accessMode}}`);

    assert.equal(
      this.element.textContent.trim(),
      `${HCP_PREFIX}?cluster_name=${clusterName}&cluster_version=${clusterVersion}&cluster_access_mode=${accessMode}`
    );
  });
});
