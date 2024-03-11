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

// organization/b4432207-bb9c-438e-a160-b98923efa979/project/4b09958c-fa91-43ab-8029-eb28d8cee9d4/hashicorp.consul.global-network-manager.cluster/test-from-api
const clusterName = 'hello';
const clusterVersion = '1.18.0';
const accessMode = 'CONSUL_ACCESS_LEVEL_GLOBAL_READ_WRITE';
const projectId = '4b09958c-fa91-43ab-8029-eb28d8cee9d4';
const realResourceId = `organization/b4432207-bb9c-438e-a160-b98923efa979/project/${projectId}/hashicorp.consul.global-network-manager.cluster/${clusterName}`;
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
    this.resourceId = realResourceId;

    await render(hbs`{{hcp-authentication-link resourceId}}`);

    assert.equal(
      this.element.textContent.trim(),
      `${HCP_PREFIX}?cluster_name=${clusterName}&cluster_version=${clusterVersion}`
    );
  });

  test('it returns correct link with invalid resourceId', async function (assert) {
    this.resourceId = 'invalid';

    await render(hbs`{{hcp-authentication-link resourceId}}`);
    assert.equal(
      this.element.textContent.trim(),
      `${HCP_PREFIX}?cluster_version=${clusterVersion}`
    );

    // not enough items in id
    this.resourceId =
      '`organization/b4432207-bb9c-438e-a160-b98923efa979/project/${projectId}/hashicorp.consul.global-network-manager.cluster`';
    await render(hbs`{{hcp-authentication-link resourceId}}`);
    assert.equal(
      this.element.textContent.trim(),
      `${HCP_PREFIX}?cluster_version=${clusterVersion}`
    );

    // value is null
    this.resourceId = null;
    await render(hbs`{{hcp-authentication-link resourceId}}`);
    assert.equal(
      this.element.textContent.trim(),
      `${HCP_PREFIX}?cluster_version=${clusterVersion}`
    );
  });

  test('it makes a URL out of a real resourceId and accessLevel, if passed', async function (assert) {
    this.resourceId = realResourceId;
    this.accessMode = accessMode;

    await render(hbs`{{hcp-authentication-link resourceId accessMode}}`);

    assert.equal(
      this.element.textContent.trim(),
      `${HCP_PREFIX}?cluster_name=${clusterName}&cluster_version=${clusterVersion}&cluster_access_mode=${accessMode}`
    );
  });
});
