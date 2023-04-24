import { module, skip } from 'qunit';
import test from 'ember-sinon-qunit/test-support/test';
import { setupTest } from 'ember-qunit';
import { run } from '@ember/runloop';
import { set } from '@ember/object';

module('Unit | Serializer | kv', function(hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function(assert) {
    const store = this.owner.lookup('service:store');
    const serializer = store.serializerFor('kv');

    assert.ok(serializer);
  });
  // TODO: Would undefined be better instead of null?
  test("it serializes records that aren't strings to null", function(assert) {
    const store = this.owner.lookup('service:store');
    const record = run(() => store.createRecord('kv', {}));

    const serializedRecord = record.serialize();
    // anything but a string ends up as null
    assert.equal(serializedRecord, null);
  });
  skip(
    'what should respondForCreate/UpdateRecord return when createRecord is called with a `false` payload'
  );
  test('respondForCreate/UpdateRecord returns a KV uid object when receiving a `true` payload', function(assert) {
    const uid = 'key/name';
    const dc = 'dc1';
    const nspace = 'default';
    const partition = 'default';
    const expected = {
      uid: JSON.stringify([partition, nspace, dc, uid]),
      Key: uid,
      Namespace: nspace,
      Partition: partition,
      Datacenter: dc,
    };
    const serializer = this.owner.lookup('serializer:kv');
    serializer.primaryKey = 'uid';
    serializer.slugKey = 'Key';
    ['respondForCreateRecord', 'respondForUpdateRecord'].forEach(function(item) {
      const actual = serializer[item](
        function(cb) {
          const headers = {
            'X-Consul-Namespace': nspace,
          };
          const body = true;
          return cb(headers, body);
        },
        {},
        {
          Key: uid,
          Datacenter: dc,
          Partition: partition,
        }
      );
      assert.deepEqual(actual, expected);
    });
  });
  test("respondForCreate/UpdateRecord returns the original object if it's not a Boolean", function(assert) {
    const uid = 'key/name';
    const dc = 'dc1';
    const nspace = 'default';
    const partition = 'default';
    const expected = {
      uid: JSON.stringify([partition, nspace, dc, uid]),
      Key: uid,
      Partition: partition,
      Namespace: nspace,
      Datacenter: dc,
    };
    const serializer = this.owner.lookup('serializer:kv');
    serializer.primaryKey = 'uid';
    serializer.slugKey = 'Key';
    ['respondForCreateRecord'].forEach(function(item) {
      const actual = serializer[item](
        function(cb) {
          const headers = {
            'X-Consul-Namespace': nspace,
            'X-Consul-Partition': partition,
          };
          const body = {
            Key: uid,
            Datacenter: dc,
          };
          return cb(headers, body);
        },
        {},
        {
          Key: uid,
          Datacenter: dc,
          Partition: partition,
        }
      );
      assert.deepEqual(actual, expected);
    });
  });
  test('serialize decodes Value if its a string', function(assert) {
    const serializer = this.owner.lookup('serializer:kv');
    set(serializer, 'decoder', {
      execute: this.stub().returnsArg(0),
    });
    //
    const expected = 'value';
    const snapshot = {
      attr: function(prop) {
        return expected;
      },
    };
    const options = {};
    const actual = serializer.serialize(snapshot, options);
    assert.equal(actual, expected);
    assert.ok(serializer.decoder.execute.calledOnce);
  });
});
