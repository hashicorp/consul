import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { HEADERS_SYMBOL as META } from 'consul-ui/utils/http/consul';

module('Unit | Serializer | application', function (hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function (assert) {
    const store = this.owner.lookup('service:store');
    const serializer = store.serializerFor('application');

    assert.ok(serializer);
  });
  test('respondForDeleteRecord returns the expected pojo structure', function (assert) {
    const store = this.owner.lookup('service:store');
    const serializer = store.serializerFor('application');
    serializer.primaryKey = 'primary-key-name';
    serializer.slugKey = 'Name';
    serializer.fingerprint = function (primary, slug, foreignValue) {
      return function (item) {
        return {
          ...item,
          ...{
            Datacenter: foreignValue,
            [primary]: item[slug],
          },
        };
      };
    };
    // adapter.uidForURL = this.stub().returnsArg(0);
    const respond = function (cb) {
      const headers = {};
      const body = true;
      return cb(headers, body);
    };
    const expected = {
      'primary-key-name': 'name',
    };
    const actual = serializer.respondForDeleteRecord(respond, {}, { Name: 'name', dc: 'dc-1' });
    assert.deepEqual(actual, expected);
    // assert.ok(adapter.uidForURL.calledOnce);
  });
  test('respondForQueryRecord returns the expected pojo structure', function (assert) {
    const store = this.owner.lookup('service:store');
    const serializer = store.serializerFor('application');
    serializer.primaryKey = 'primary-key-name';
    serializer.slugKey = 'Name';
    serializer.fingerprint = function (primary, slug, foreignValue) {
      return function (item) {
        return {
          ...item,
          ...{
            Datacenter: foreignValue,
            [primary]: item[slug],
          },
        };
      };
    };
    const expected = {
      Datacenter: 'dc-1',
      Name: 'name',
      [META]: {},
      'primary-key-name': 'name',
    };
    const respond = function (cb) {
      const headers = {};
      const body = {
        Name: 'name',
      };
      return cb(headers, body);
    };
    const actual = serializer.respondForQueryRecord(respond, { Name: 'name', dc: 'dc-1' });
    assert.deepEqual(actual, expected);
  });
  test('respondForQuery returns the expected pojo structure', function (assert) {
    const store = this.owner.lookup('service:store');
    const serializer = store.serializerFor('application');
    serializer.primaryKey = 'primary-key-name';
    serializer.slugKey = 'Name';
    serializer.fingerprint = function (primary, slug, foreignValue) {
      return function (item) {
        return {
          ...item,
          ...{
            Datacenter: foreignValue,
            [primary]: item[slug],
          },
        };
      };
    };
    const expected = [
      {
        Datacenter: 'dc-1',
        Name: 'name1',
        'primary-key-name': 'name1',
      },
      {
        Datacenter: 'dc-1',
        Name: 'name2',
        'primary-key-name': 'name2',
      },
    ];
    const respond = function (cb) {
      const headers = {};
      const body = [
        {
          Name: 'name1',
        },
        {
          Name: 'name2',
        },
      ];
      return cb(headers, body);
    };
    const actual = serializer.respondForQuery(respond, { Name: 'name', dc: 'dc-1' });
    assert.deepEqual(actual, expected);
    // assert.ok(adapter.uidForURL.calledTwice);
  });
});
