/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { nspaceJSON, nspaceYAML, nspaceEndpoint } from 'consul-ui/tests/helpers/nspace-template';

module('Unit | Test helper | nspace-template', function () {
  module('nspaceJSON', function () {
    test('it substitutes the namespace', function (assert) {
      assert.deepEqual(nspaceJSON('{"Namespace":"@namespace"}', 'team-1'), {
        Namespace: 'team-1',
      });
    });
    [undefined, ''].forEach((nspace) => {
      test(`it falls back to default for ${JSON.stringify(nspace)}`, function (assert) {
        assert.deepEqual(nspaceJSON('{"Namespace":"@namespace"}', nspace), {
          Namespace: 'default',
        });
      });
    });
  });

  module('nspaceEndpoint', function () {
    const url = '/v1/catalog/services?dc=dc-1&ns=@namespace&merge-central-config';
    const always = '/v1/internal/ui/services?dc=dc-1&ns=@!namespace';

    test('it sets ns= to the namespace when one is set', function (assert) {
      assert.strictEqual(
        nspaceEndpoint(url, 'team-1', { nspacesEnabled: true }),
        '/v1/catalog/services?dc=dc-1&ns=team-1&merge-central-config'
      );
    });
    [undefined, ''].forEach((nspace) => {
      test(`it drops ns= when no namespace is set (${JSON.stringify(nspace)})`, function (assert) {
        assert.strictEqual(
          nspaceEndpoint(url, nspace, { nspacesEnabled: true }),
          '/v1/catalog/services?dc=dc-1&merge-central-config'
        );
      });
    });
    test('it drops a leading ns= parameter without leaving a dangling &', function (assert) {
      assert.strictEqual(
        nspaceEndpoint('/v1/x?ns=@namespace&dc=dc-1', undefined, { nspacesEnabled: true }),
        '/v1/x?dc=dc-1'
      );
    });
    test('@!namespace is always set under Enterprise, defaulting to default', function (assert) {
      assert.strictEqual(
        nspaceEndpoint(always, 'team-1', { nspacesEnabled: true }),
        '/v1/internal/ui/services?dc=dc-1&ns=team-1'
      );
      assert.strictEqual(
        nspaceEndpoint(always, undefined, { nspacesEnabled: true }),
        '/v1/internal/ui/services?dc=dc-1&ns=default'
      );
    });
    test('@!namespace and ns=* are removed under CE', function (assert) {
      assert.strictEqual(
        nspaceEndpoint(always, 'team-1', { nspacesEnabled: false }),
        '/v1/internal/ui/services?dc=dc-1'
      );
      assert.strictEqual(
        nspaceEndpoint('/v1/x?dc=dc-1&ns=*', undefined, { nspacesEnabled: false }),
        '/v1/x?dc=dc-1'
      );
    });
  });

  module('nspaceYAML', function () {
    const yaml = [
      '- Name: a',
      '  Namespace: @namespace',
      '- Name: b',
      '  Namespace: @!namespace',
    ].join('\n');

    test('it substitutes the namespace under Enterprise', function (assert) {
      assert.deepEqual(nspaceYAML(yaml, 'team-1', { nspacesEnabled: true }), [
        { Name: 'a', Namespace: 'team-1' },
        { Name: 'b', Namespace: 'team-1' },
      ]);
    });
    [undefined, ''].forEach((nspace) => {
      test(`with no namespace (${JSON.stringify(
        nspace
      )}) the Namespace key is dropped for @namespace and @!namespace defaults`, function (assert) {
        assert.deepEqual(nspaceYAML(yaml, nspace, { nspacesEnabled: true }), [
          { Name: 'a' },
          { Name: 'b', Namespace: 'default' },
        ]);
      });
    });
    test('under CE @!namespace is removed', function (assert) {
      assert.deepEqual(nspaceYAML(yaml, undefined, { nspacesEnabled: false }), [
        { Name: 'a' },
        { Name: 'b' },
      ]);
    });
    test('under CE a set namespace is still substituted for @namespace', function (assert) {
      assert.deepEqual(nspaceYAML('Namespace: @namespace', 'team-1', { nspacesEnabled: false }), {
        Namespace: 'team-1',
      });
    });
    test('query strings in yaml are rewritten like endpoints', function (assert) {
      const str = ['- /v1/a?dc=dc-1&ns=@!namespace', '- /v1/namespaces'].join('\n');
      assert.deepEqual(nspaceYAML(str, 'team-1', { nspacesEnabled: true }), [
        '/v1/a?dc=dc-1&ns=team-1',
        '/v1/namespaces',
      ]);
      assert.deepEqual(nspaceYAML(str, 'team-1', { nspacesEnabled: false }), ['/v1/a?dc=dc-1']);
    });
  });
});
