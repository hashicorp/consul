/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, click, fillIn } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { LANGUAGES } from 'consul-ui/components/consul/kv/form/index';

module('Integration | Component | consul/kv/form', function (hooks) {
  setupRenderingTest(hooks);

  test('LANGUAGES array includes TOML option', function (assert) {
    const toml = LANGUAGES.find((l) => l.value === 'toml');
    assert.ok(toml, 'TOML language option exists');
    assert.strictEqual(toml.label, 'TOML');
    assert.strictEqual(toml.value, 'toml');
  });

  test('LANGUAGES array includes all expected options', function (assert) {
    const values = LANGUAGES.map((l) => l.value);
    assert.ok(values.includes('json'), 'includes json');
    assert.ok(values.includes('yaml'), 'includes yaml');
    assert.ok(values.includes('hcl'), 'includes hcl');
    assert.ok(values.includes('toml'), 'includes toml');
    assert.ok(values.includes('ruby'), 'includes ruby');
    assert.ok(values.includes('shell'), 'includes shell');
  });
});
