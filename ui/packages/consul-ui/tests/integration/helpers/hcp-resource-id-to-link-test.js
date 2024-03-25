/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { setupRenderingTest } from 'ember-qunit';
import { HCP_PREFIX } from 'consul-ui/helpers/hcp-resource-id-to-link';

// organization/b4432207-bb9c-438e-a160-b98923efa979/project/4b09958c-fa91-43ab-8029-eb28d8cee9d4/hashicorp.consul.global-network-manager.cluster/test-from-api
const clusterName = 'hello';
const projectId = '4b09958c-fa91-43ab-8029-eb28d8cee9d4';
const realResourceId = `organization/b4432207-bb9c-438e-a160-b98923efa979/project/${projectId}/hashicorp.consul.global-network-manager.cluster/${clusterName}`;
module('Integration | Helper | hcp-resource-id-to-link', function (hooks) {
  setupRenderingTest(hooks);
  test('it makes a URL out of a real resourceId', async function (assert) {
    this.resourceId = realResourceId;

    await render(hbs`{{hcp-resource-id-to-link resourceId}}`);

    assert.equal(
      this.element.textContent.trim(),
      `${HCP_PREFIX}/${clusterName}?project_id=${projectId}`
    );
  });

  test('it returns empty string with invalid resourceId', async function (assert) {
    this.resourceId = 'invalid';

    await render(hbs`{{hcp-resource-id-to-link resourceId}}`);
    assert.equal(this.element.textContent.trim(), '');

    // not enough items in id
    this.resourceId =
      '`organization/b4432207-bb9c-438e-a160-b98923efa979/project/${projectId}/hashicorp.consul.global-network-manager.cluster`';
    await render(hbs`{{hcp-resource-id-to-link resourceId}}`);
    assert.equal(this.element.textContent.trim(), '');
  });
});
