/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import domSibling from 'consul-ui/utils/dom/sibling';
import { module, test } from 'qunit';

module('Unit | Utility | dom/sibling', function () {
  test('it returns the next sibling if it matches the requested nodeName', function (assert) {
    const expected = {
      nodeType: 1,
      nodeName: 'H1',
    };
    const actual = domSibling(
      {
        nextSibling: expected,
      },
      'h1'
    );
    assert.deepEqual(actual, expected);
  });
  test('it returns the next sibling from a list of nodes if it matches the requested nodeName', function (assert) {
    const expected = {
      nodeType: 1,
      nodeName: 'H1',
    };
    const nodes = {
      nodeType: 3,
      nodeName: '#text',
      nextSibling: {
        nodeType: 4,
        nodeName: '#cdata-section',
        nextSibling: expected,
      },
    };
    const actual = domSibling(
      {
        nextSibling: nodes,
      },
      'h1'
    );
    assert.deepEqual(actual, expected);
  });
  test("it returns the null from a list of nodes if it can't match", function (assert) {
    let expected;
    const nodes = {
      nodeType: 3,
      nodeName: '#text',
      nextSibling: {
        nodeType: 4,
        nodeName: '#cdata-section',
        nextSibling: {
          nodeType: 1,
          nodeName: 'p',
        },
      },
    };
    const actual = domSibling(
      {
        nextSibling: nodes,
      },
      'h1'
    );
    assert.deepEqual(actual, expected);
  });
});
