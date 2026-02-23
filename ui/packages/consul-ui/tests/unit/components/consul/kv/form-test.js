/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { LANGUAGES } from 'consul-ui/components/consul/kv/form/index';
import { module, test } from 'qunit';

module('Unit | Component | consul/kv/form', function () {
  test('LANGUAGES includes toml', function (assert) {
    const toml = LANGUAGES.find((lang) => lang.value === 'toml');
    assert.ok(toml, 'TOML is present in the language list');
    assert.strictEqual(toml.label, 'TOML', 'TOML has the correct label');
  });

  test('LANGUAGES includes all expected languages', function (assert) {
    const expectedValues = ['json', 'yaml', 'hcl', 'toml', 'ruby', 'shell'];
    const actualValues = LANGUAGES.map((lang) => lang.value);
    expectedValues.forEach((value) => {
      assert.ok(actualValues.includes(value), `Language list includes ${value}`);
    });
  });

  test('LANGUAGES has correct label-value pairs', function (assert) {
    const expected = {
      json: 'JSON',
      yaml: 'YAML',
      hcl: 'HCL',
      toml: 'TOML',
      ruby: 'Ruby',
      shell: 'Shell',
    };
    LANGUAGES.forEach((lang) => {
      assert.strictEqual(
        lang.label,
        expected[lang.value],
        `${lang.value} has label ${expected[lang.value]}`
      );
    });
  });
});
