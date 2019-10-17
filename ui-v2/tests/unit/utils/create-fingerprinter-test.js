import createFingerprinter from 'consul-ui/utils/create-fingerprinter';
import { module, test } from 'qunit';

module('Unit | Utility | create fingerprinter', function() {
  test("fingerprint returns a 'unique' fingerprinted object based on primary, slug and foreign keys", function(assert) {
    const obj = {
      ID: 'slug',
    };
    const expected = {
      Datacenter: 'dc',
      Namespace: 'default',
      ID: 'slug',
      uid: '["default","dc","slug"]',
    };
    const fingerprint = createFingerprinter('Datacenter', 'Namespace', 'default');
    const actual = fingerprint('uid', 'ID', 'dc')(obj);
    assert.deepEqual(actual, expected);
  });
  test("fingerprint throws an error if it can't find a foreignKey", function(assert) {
    const fingerprint = createFingerprinter('Datacenter', 'Namespace', 'default');
    [undefined, null].forEach(function(item) {
      assert.throws(function() {
        fingerprint('uid', 'ID', item);
      }, /missing foreignKey/);
    });
  });
  test("fingerprint throws an error if it can't find a slug", function(assert) {
    const fingerprint = createFingerprinter('Datacenter', 'Namespace', 'default');
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
