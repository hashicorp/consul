/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/show.feature.

import { module } from 'qunit';
import { waitUntil } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  setUiConfig,
  setPermissions,
  setLatency,
  visit,
  page,
  assertVisibleCount,
  assertProperties,
  assertPropertiesVertically,
} from 'consul-ui/tests/helpers/acceptance';

const visitService = function (nspace) {
  return visit('service', { dc: 'dc1', service: 'service-0' }, { nspace });
};

const instance = (status, id, port, address, nodeAddress, meta = {}) => ({
  Checks: [{ Status: status }],
  Service: { Kind: null, ID: id, Port: port, Address: address, ...meta.service },
  Node: { Address: nodeAddress, ...meta.node },
});

const external = (source) => ({ service: { Meta: { 'external-source': source } } });
const synthetic = (value) => ({ node: { Meta: { 'synthetic-node': value } } });
const both = (a, b) => ({ service: a.service, node: b.node });

module('Acceptance | dc / services / show: Show Service', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 1, 'dc1');
  });

  nspaceScenario(
    'Scenario: Given a service with an external source, the logo is displayed',
    async function (assert, nspace) {
      createModels('node', 1);
      createModels('service', 1, [
        { Service: { Kind: null, Tags: ['Tag1', 'Tag2'], Meta: { 'external-source': 'consul' } } },
      ]);

      await visitService(nspace);
      assert.strictEqual(page().externalSource, 'consul');
      assert.equal(document.title, 'service-0 - Consul');
    }
  );

  nspaceScenario(
    "Scenario: Given a service with an 'unsupported' external source, there is no logo",
    async function (assert, nspace) {
      createModels('node', 1);
      createModels('service', 1, [
        {
          Service: {
            Kind: null,
            Tags: ['Tag1', 'Tag2'],
            Meta: { 'external-source': 'not-supported' },
          },
        },
      ]);

      await visitService(nspace);
      assert.dom('.title [data-test-external-source]').doesNotExist('there is no logo');
    }
  );

  nspaceScenario(
    'Scenario: Given various services with various tags, all tags are displayed',
    async function (assert, nspace) {
      createModels('node', 3);
      createModels('service', 1, [
        { Service: { Kind: null, Tags: ['Tag1', 'Tag2'] } },
        { Service: { Kind: null, Tags: ['Tag3', 'Tag1'] } },
        { Service: { Kind: null, Tags: ['Tag2', 'Tag3'] } },
      ]);

      await visitService(nspace);
      await page().tabs.tags();
      assert.dom('[data-test-tags] span:nth-child(1)').includesText('Tag1');
      assert.dom('[data-test-tags] span:nth-child(2)').includesText('Tag2');
      assert.dom('[data-test-tags] span:nth-child(3)').includesText('Tag3');
    }
  );

  nspaceScenario(
    'Scenario: Given various services the various nodes on their instances are displayed',
    async function (assert, nspace) {
      createModels('node', 3);
      createModels('service', 1, [
        instance('critical', 'passing-service-8080', 8080, '1.1.1.1', '1.2.2.2'),
        instance('warning', 'service-8000', 8000, '2.2.2.2', '2.3.3.3'),
        instance('passing', 'service-8888', 8888, '3.3.3.3', '3.4.4.4'),
      ]);

      await visitService(nspace);
      await page().tabs.instances();
      assertProperties(
        assert,
        page().instances,
        'address',
        ['1.1.1.1:8080', '2.2.2.2:8000', '3.3.3.3:8888'],
        'instances'
      );
    }
  );

  nspaceScenario(
    'Scenario: Given a combination of sources I should see them all on the instances',
    async function (assert, nspace) {
      createModels('node', 3);
      createModels('service', 1, [
        instance(
          'critical',
          'passing-service-8080',
          8080,
          '1.1.1.1',
          '1.2.2.2',
          both(external('kubernetes'), synthetic(false))
        ),
        instance(
          'passing',
          'service-8000',
          8000,
          '2.2.2.2',
          '2.3.3.3',
          both(external('kubernetes'), synthetic(false))
        ),
        instance(
          'passing',
          'service-8888',
          8888,
          '3.3.3.3',
          '3.4.4.4',
          both(external('vault'), synthetic(false))
        ),
      ]);

      await visitService(nspace);
      await page().tabs.instances();
      assertPropertiesVertically(
        assert,
        page().instances,
        'externalSource',
        ['kubernetes', 'kubernetes', 'vault'],
        'instances'
      );
      assertProperties(
        assert,
        page().instances,
        'nodeName',
        ['node-0', 'node-1', 'node-2'],
        'instances'
      );
      assert.ok(page().instances.objectAt(0).nodeChecks, 'shows node checks');
    }
  );

  nspaceScenario(
    'Scenario: Given instances share the same external source, it is shown on every instance row',
    async function (assert, nspace) {
      createModels('node', 3);
      createModels('service', 1, [
        instance(
          'critical',
          'passing-service-8080',
          8080,
          '1.1.1.1',
          '1.2.2.2',
          both(external('kubernetes'), synthetic(true))
        ),
        instance(
          'passing',
          'service-8000',
          8000,
          '2.2.2.2',
          '2.3.3.3',
          both(external('kubernetes'), synthetic(true))
        ),
        instance('passing', 'service-8888', 8888, '3.3.3.3', '3.4.4.4', external('kubernetes')),
      ]);

      await visitService(nspace);
      await page().tabs.instances();
      assert.strictEqual(page().externalSource, 'kubernetes');
      assertPropertiesVertically(
        assert,
        page().instances,
        'externalSource',
        ['kubernetes', 'kubernetes', 'kubernetes'],
        'instances'
      );
    }
  );

  nspaceScenario(
    'Scenario: Given one agentless instance, it should not show node health checks or node name',
    async function (assert, nspace) {
      createModels('node', 1);
      createModels('service', 1, [
        instance(
          'critical',
          'passing-service-8080',
          8080,
          '1.1.1.1',
          '1.2.2.2',
          both(external('kubernetes'), synthetic(true))
        ),
      ]);

      await visitService(nspace);
      await page().tabs.instances();
      const rows = '.consul-service-instance-table tbody tr';
      assert.dom(`${rows} [data-test-node-health-checks]`).doesNotExist('no node checks');
      assert.dom(`${rows} [data-test-node-name]`).doesNotExist('no node name');
    }
  );

  nspaceScenario(
    'Scenario: Given a dashboard template has been set',
    async function (assert, nspace) {
      setUiConfig({
        dashboard_url_templates: {
          service: 'https://something.com?{{Service.Name}}&{{Datacenter}}',
        },
      });

      await visitService(nspace);
      // The external dashboard link should use the Service.Name not the ID
      assert.strictEqual(page().dashboardAnchor.href, 'https://something.com?service-0&dc1');
    }
  );

  nspaceScenario('Scenario: With no access to service', async function (assert, nspace) {
    setPermissions({ service: { read: false } });

    await visitService(nspace);
    assert.strictEqual(page().error.status, '403');
  });

  nspaceScenario(
    'Scenario: When access is removed from a service',
    async function (assert, nspace) {
      createModels('node', 1);
      createModels('service', 1);
      setLatency(100);
      setPermissions({ service: { read: true } });

      await visitService(nspace);
      await page().tabs.instances();
      assertVisibleCount(assert, page().instances, 1, 'instance models');

      setPermissions({ service: { read: false } });
      // authorization requests are not blocking so we just wait until the next
      // service blocking query responds
      const noAccess = () => {
        const notification = document.querySelector('[data-notification]');
        return notification && notification.textContent.includes('no longer have access');
      };
      try {
        await waitUntil(noAccess, { timeout: 5000 });
      } catch (e) {
        // reported by the assertions below
      }
      assert.dom('[data-notification]').includesText('no longer have access');
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
      assert.strictEqual(page().error.status, '403');
    }
  );
});
