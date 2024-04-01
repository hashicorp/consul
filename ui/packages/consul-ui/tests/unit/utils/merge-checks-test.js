/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import mergeChecks from 'consul-ui/utils/merge-checks';
import { module, test } from 'qunit';

module('Unit | Utility | merge-checks', function () {
  test('it works', function (assert) {
    assert.expect(4);
    [
      {
        desc: 'One list of checks, not exposed',
        exposed: false,
        checks: [
          [
            {
              ServiceName: 'service-0',
              CheckID: 'service-0-check-0',
              Node: 'node-0',
              Exposable: false,
            },
          ],
        ],
        expected: [
          {
            ServiceName: 'service-0',
            CheckID: 'service-0-check-0',
            Node: 'node-0',
            Exposable: false,
          },
        ],
      },
      {
        desc: 'One list of checks, exposed',
        exposed: true,
        checks: [
          [
            {
              ServiceName: 'service-0',
              CheckID: 'service-0-check-0',
              Node: 'node-0',
              Exposable: true,
            },
          ],
        ],
        expected: [
          {
            ServiceName: 'service-0',
            CheckID: 'service-0-check-0',
            Node: 'node-0',
            Exposable: true,
            Exposed: true,
          },
        ],
      },
      {
        desc: 'Two lists of checks, not exposed',
        exposed: false,
        checks: [
          [
            {
              ServiceName: 'service-0',
              CheckID: 'service-0-check-0',
              Node: 'node-0',
              Exposable: true,
            },
          ],
          [
            {
              ServiceName: 'service-0-proxy',
              CheckID: 'service-0-proxy-check-0',
              Node: 'node-0',
              Exposable: true,
            },
          ],
        ],
        expected: [
          {
            ServiceName: 'service-0',
            CheckID: 'service-0-check-0',
            Node: 'node-0',
            Exposable: true,
          },
          {
            ServiceName: 'service-0-proxy',
            CheckID: 'service-0-proxy-check-0',
            Node: 'node-0',
            Exposable: true,
          },
        ],
      },
      {
        desc: 'Two lists of checks, with one duplicate node checks, not exposed',
        exposed: false,
        checks: [
          [
            {
              ServiceName: 'service-0',
              CheckID: 'service-0-check-0',
              Node: 'node-0',
              Exposable: true,
            },
            {
              ServiceName: '',
              CheckID: 'service-0-check-1',
              Node: 'node-0',
              Exposable: true,
            },
          ],
          [
            {
              ServiceName: 'service-0-proxy',
              CheckID: 'service-0-proxy-check-0',
              Node: 'node-0',
              Exposable: true,
            },
            {
              ServiceName: '',
              CheckID: 'service-0-check-1',
              Node: 'node-0',
              Exposable: true,
            },
          ],
        ],
        expected: [
          {
            ServiceName: 'service-0',
            CheckID: 'service-0-check-0',
            Node: 'node-0',
            Exposable: true,
          },
          {
            ServiceName: '',
            CheckID: 'service-0-check-1',
            Node: 'node-0',
            Exposable: true,
          },
          {
            ServiceName: 'service-0-proxy',
            CheckID: 'service-0-proxy-check-0',
            Node: 'node-0',
            Exposable: true,
          },
        ],
      },
    ].forEach((spec) => {
      const actual = mergeChecks(spec.checks, spec.exposed);
      assert.deepEqual(actual, spec.expected);
    });
  });
});
