import createFingerprinter from 'consul-ui/utils/create-fingerprinter';
import { module, test } from 'qunit';

module('Unit | Utility | create fingerprinter', function() {
  test("fingerprint returns a 'unique' fingerprinted object based on primary, slug and foreign keys", function(assert) {
    const obj = {
      ID: 'slug',
      Namespace: 'namespace',
    };
    const expected = {
      Datacenter: 'dc',
      Namespace: 'namespace',
      Partition: 'partition',
      ID: 'slug',
      uid: '["partition","namespace","dc","slug"]',
    };
    const fingerprint = createFingerprinter('Datacenter', 'Namespace', 'Partition');
    const actual = fingerprint('uid', 'ID', 'dc', 'namespace', 'partition')(obj);
    assert.deepEqual(actual, expected);
  });
  test("fingerprint returns a 'unique' fingerprinted object based on primary, slug and foreign keys, and uses default namespace if none set", function(assert) {
    const obj = {
      ID: 'slug',
    };
    const expected = {
      Datacenter: 'dc',
      Namespace: 'default',
      Partition: 'default',
      ID: 'slug',
      uid: '["default","default","dc","slug"]',
    };
    const fingerprint = createFingerprinter('Datacenter', 'Namespace', 'Partition');
    const actual = fingerprint('uid', 'ID', 'dc', 'default', 'default')(obj);
    assert.deepEqual(actual, expected);
  });
  test("fingerprint throws an error if it can't find a foreignKey", function(assert) {
    const fingerprint = createFingerprinter('Datacenter', 'Namespace', 'Partition');
    [undefined, null].forEach(function(item) {
      assert.throws(function() {
        fingerprint('uid', 'ID', item);
      }, /missing foreignKey/);
    });
  });
  test("fingerprint throws an error if it can't find a slug", function(assert) {
    const fingerprint = createFingerprinter('Datacenter', 'Namespace', 'Partition');
    [
      {},
      {
        ID: null,
      },
    ].forEach(function(item) {
      assert.throws(function() {
        fingerprint('uid', 'ID', 'dc')(item);
      }, /missing slug/);
    });
  });
});
