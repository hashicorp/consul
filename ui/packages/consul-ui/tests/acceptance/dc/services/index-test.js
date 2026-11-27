/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  disableACLs,
  visit,
  page,
  currentURL,
  assertVisibleCount,
  assertProperties,
} from 'consul-ui/tests/helpers/acceptance';

const checks = (critical) => ({ ChecksPassing: 0, ChecksWarning: 0, ChecksCritical: critical });

const visitServices = async function (assert, nspace) {
  await visit('services', { dc: 'dc-1' }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services'));
  assert.equal(document.title, 'Services - Consul');
};

module('Acceptance | dc / services / index: List Services', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 1, 'dc-1');
  });

  nspaceScenario(
    'Scenario: Viewing the service list page with services',
    async function (assert, nspace) {
      const sources = ['consul', 'nomad', 'terraform', 'kubernetes', 'aws'];
      const services = [];
      sources.forEach((source, i) => {
        services.push({
          Name: `Service-${i}`,
          ExternalSources: [source],
          ...checks(10 - i),
          Kind: null,
        });
        services.push({ Name: `Service-${i}-proxy`, Kind: 'connect-proxy' });
      });
      services[9] = { ...services[9], ...checks(5) };
      createModels('service', 10, services);

      await visitServices(assert, nspace);
      assertVisibleCount(assert, page().services, 5, 'service models');
      assertProperties(assert, page().services, 'externalSource', sources, 'services');
    }
  );

  nspaceScenario(
    'Scenario: Viewing the service list page with gateways',
    async function (assert, nspace) {
      createModels('service', 4, [
        { Name: 'Service-0-proxy', Kind: 'connect-proxy', ...checks(3) },
        { Name: 'Service-1-ingress-gateway', Kind: 'ingress-gateway', ...checks(2) },
        { Name: 'Service-2-terminating-gateway', Kind: 'terminating-gateway', ...checks(1) },
        { Name: 'Service-3-api-gateway', Kind: 'api-gateway', ...checks(1) },
      ]);

      await visitServices(assert, nspace);
      assertVisibleCount(assert, page().services, 3, 'service models');
      assertProperties(
        assert,
        page().services,
        'kind',
        ['Ingress gateway', 'Terminating gateway', 'API gateway'],
        'services'
      );
    }
  );

  nspaceScenario('Scenario: View a Service in mesh', async function (assert, nspace) {
    createModels('service', 3, [
      {
        Name: 'Service-0',
        Kind: null,
        ConnectedWithProxy: true,
        ConnectedWithGateway: true,
        ...checks(2),
      },
      { Name: 'Service-0-proxy', Kind: 'connect-proxy' },
      {
        Name: 'Service-1',
        Kind: null,
        ConnectedWithProxy: false,
        ConnectedWithGateway: false,
        ...checks(1),
      },
    ]);

    await visitServices(assert, nspace);
    assertVisibleCount(assert, page().services, 2, 'service models');
    assert.ok(page().services.objectAt(0).mesh, 'the first service is in the mesh');
    assert.notOk(page().services.objectAt(1).mesh, 'the second service is not in the mesh');
  });

  nspaceScenario(
    "Scenario: View a Service's Associated Service count",
    async function (assert, nspace) {
      createModels('service', 3, [
        { Name: 'Service-0', Kind: null, ...checks(2) },
        { Name: 'Service-0-proxy', Kind: 'connect-proxy', ...checks(1) },
        {
          Name: 'Service-1',
          Kind: 'ingress-gateway',
          ...checks(1),
          GatewayConfig: { AssociatedServiceCount: 345 },
        },
      ]);

      await visitServices(assert, nspace);
      assertVisibleCount(assert, page().services, 2, 'service models');
      assert.notOk(page().services.objectAt(0).associatedServiceCount, 'no count on a service');
      assert.ok(page().services.objectAt(1).associatedServiceCount, 'a count on the gateway');
    }
  );

  nspaceScenario(
    'Scenario: Viewing the services index page with no services and ACLs enabled',
    async function (assert, nspace) {
      createModels('service', 0);

      await visitServices(assert, nspace);
      assertVisibleCount(assert, page().services, 0, 'service models');
      assert
        .dom('.empty-state p')
        .includesText(
          "There don't seem to be any registered services in this Consul cluster, or you may not have service:read and node:read access to this view. Use Terraform, Kubernetes CRDs, Vault, or the Consul CLI to register Services."
        );
      assert.dom('[data-test-empty-state-login]').exists();
    }
  );

  nspaceScenario(
    'Scenario: Viewing the services index page with no services and ACLs disabled',
    async function (assert, nspace) {
      disableACLs();
      createModels('service', 0);

      await visitServices(assert, nspace);
      assertVisibleCount(assert, page().services, 0, 'service models');
      assert
        .dom('.empty-state p')
        .includesText("There don't seem to be any registered services in this Consul cluster.");
      assert.dom('[data-test-empty-state-login]').doesNotExist();
    }
  );
});
