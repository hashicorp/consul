/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { processIpAddress } from 'consul-ui/utils/process-ip-address';
import { module, test } from 'qunit';

module('Unit | Utility | Process Ip Address', function () {
  test('Returns as it is for ipv4 and already collapsed', function (assert) {
    let result = processIpAddress('192.168.1.1');
    assert.equal(result, '192.168.1.1');

    assert.equal(processIpAddress('255.255.255.255'), '255.255.255.255');

    assert.equal(processIpAddress('2001:db8::ff00:42:8329'), '[2001:db8::ff00:42:8329]');

    assert.equal(processIpAddress('::1'), '[::1]');

    assert.equal(processIpAddress('fe80::202:b3ff:fe1e:8329'), '[fe80::202:b3ff:fe1e:8329]');

    assert.equal(processIpAddress('::'), '[::]');
  });

  test('Returns null for invalid IP address', function (assert) {
    assert.equal(processIpAddress('2001::85a3::8a2e:370:7334'), null);

    assert.equal(processIpAddress('2001:db8:0:0:0:0:0:0:1:2'), null);
    assert.equal(processIpAddress('2001:db8:g::1'), null);
    assert.equal(processIpAddress('2001:db8:1::2:3:4:5:6'), null);
  });

  test('Returns collapsed IP address', function (assert) {
    assert.equal(
      processIpAddress('2001:0db8:0000:0000:0000:ff00:0042:8329'),
      '[2001:db8::ff00:42:8329]'
    );

    assert.equal(processIpAddress('2001:db8:0:0:0:ff00:42:8329'), '[2001:db8::ff00:42:8329]');

    assert.equal(processIpAddress('2001:db8::ff00:42:8329'), '[2001:db8::ff00:42:8329]');

    assert.equal(processIpAddress('fe80::202:b3ff:fe1e:8329'), '[fe80::202:b3ff:fe1e:8329]');
  });
});
