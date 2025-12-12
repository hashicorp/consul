/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { processIpAddress } from 'consul-ui/utils/process-ip-address';
import { module, test } from 'qunit';

module('Unit | Utility | Process Ip Address', function () {
  test('Returns as it is for ipv4 and already collapsed', function (assert) {
    let result = processIpAddress('192.168.1.1');
    assert.strictEqual(result, '192.168.1.1');

    assert.strictEqual(processIpAddress('255.255.255.255'), '255.255.255.255');

    assert.strictEqual(processIpAddress('2001:db8::ff00:42:8329'), '[2001:db8::ff00:42:8329]');

    assert.strictEqual(processIpAddress('::1'), '[::1]');

    assert.strictEqual(processIpAddress('fe80::202:b3ff:fe1e:8329'), '[fe80::202:b3ff:fe1e:8329]');

    assert.strictEqual(processIpAddress('::'), '[::]');
  });

  test('Returns null for invalid IP address', function (assert) {
    assert.strictEqual(processIpAddress('2001::85a3::8a2e:370:7334'), null);

    assert.strictEqual(processIpAddress('2001:db8:0:0:0:0:0:0:1:2'), null);
    assert.strictEqual(processIpAddress('2001:db8:g::1'), null);
    assert.strictEqual(processIpAddress('2001:db8:1::2:3:4:5:6'), null);
  });

  test('Returns collapsed IP address', function (assert) {
    assert.strictEqual(
      processIpAddress('2001:0db8:0000:0000:0000:ff00:0042:8329'),
      '[2001:db8::ff00:42:8329]'
    );

    assert.strictEqual(processIpAddress('2001:db8:0:0:0:ff00:42:8329'), '[2001:db8::ff00:42:8329]');

    assert.strictEqual(processIpAddress('2001:db8::ff00:42:8329'), '[2001:db8::ff00:42:8329]');

    assert.strictEqual(processIpAddress('fe80::202:b3ff:fe1e:8329'), '[fe80::202:b3ff:fe1e:8329]');
  });

  test('Returns as it is for valid FQDNs', function (assert) {
    assert.strictEqual(processIpAddress('example.com'), 'example.com');
    assert.strictEqual(processIpAddress('sub.domain.example.com'), 'sub.domain.example.com');
    assert.strictEqual(processIpAddress('a-b-c.domain.co.uk'), 'a-b-c.domain.co.uk');
    assert.strictEqual(processIpAddress('xn--d1acufc.xn--p1ai'), 'xn--d1acufc.xn--p1ai'); // punycode
    assert.strictEqual(processIpAddress('localhost'), 'localhost');
    assert.strictEqual(processIpAddress('my-service.local'), 'my-service.local');
  });
});
