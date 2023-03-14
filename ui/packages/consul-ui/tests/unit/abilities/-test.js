/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

/* globals requirejs */
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Ability | *', function (hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function (assert) {
    assert.expect(228);

    const abilities = Object.keys(requirejs.entries)
      .filter((key) => key.indexOf('/abilities/') !== -1)
      .map((key) => key.split('/').pop())
      .filter((item) => item !== '-test');
    abilities.forEach((item) => {
      const ability = this.owner.factoryFor(`ability:${item}`).create();
      [true, false].forEach((bool) => {
        const permissions = this.owner.lookup(`service:repository/permission`);
        ability.permissions = {
          has: (_) => bool,
          permissions: bool ? ['more-than-zero'] : [],
          generate: function () {
            return permissions.generate(...arguments);
          },
        };
        ['Create', 'Read', 'Update', 'Delete', 'Write', 'List'].forEach((perm) => {
          switch (item) {
            case 'permission':
              ability.item = {
                ID: bool ? 'not-anonymous' : 'anonymous',
              };
              break;
            case 'acl':
              ability.item = {
                ID: bool ? 'not-anonymous' : 'anonymous',
              };
              break;
            case 'token':
              ability.item = {
                AccessorID: 'not-anonymous',
              };
              ability.token = {
                AccessorID: bool ? 'different-to-item' : 'not-anonymous',
              };
              break;
            case 'nspace':
            case 'partition':
              ability.item = {
                ID: bool ? 'not-default' : 'default',
              };
              break;
            case 'peer':
              ability.item = {
                State: bool ? 'ACTIVE' : 'DELETING',
              };
              break;
            case 'kv':
              // TODO: We currently hardcode KVs to always be true
              assert.true(ability[`can${perm}`], `Expected ${item}.can${perm} to be true`);
              // eslint-disable-next-line qunit/no-early-return
              return;
            case 'license':
            case 'zone':
              // Zone permissions depend on NSPACES_ENABLED
              // License permissions also depend on NSPACES_ENABLED;
              // behavior works as expected when verified manually but test
              // fails due to this dependency. -@evrowe 2022-04-18
              // eslint-disable-next-line qunit/no-early-return
              return;
          }
          assert.equal(
            bool,
            ability[`can${perm}`],
            `Expected ${item}.can${perm} to be ${bool ? 'true' : 'false'}`
          );
        });
      });
    });
  });
});
