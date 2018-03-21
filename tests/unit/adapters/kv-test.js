import { module, skip } from 'qunit';
import { setupTest } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import _super from 'consul-ui/tests/helpers/stub-super';

module('Unit | Adapter | kv', function(hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    assert.ok(adapter);
  });
  test('handleResponse', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const expected = 'key/name';
    const url = `/v1/kv/${expected}?dc=dc1`;
    const urlForCreateRecord = this.stub(adapter, 'urlForCreateRecord');
    urlForCreateRecord.returns(url);
    const it = _super(adapter, function(status, headers, response, requestData) {
      return response;
    });

    it("return's a KV pojo when createRecord is called with a `true` payload", function() {
      const deep = {
        Key: expected,
        Datacenter: '',
      };
      const actual = adapter.handleResponse(200, {}, true, { url: url });
      assert.deepEqual(actual, deep);
    });
    it("return's the original payload if it's not a Boolean", function() {
      const expected = [];
      const actual = adapter.handleResponse(200, {}, expected, { url: url });
      assert.deepEqual(actual, expected);
    });
  });
  skip("what's should handleResponse return when createRecord is called with a `false` payload");
  test('dataForRequest returns', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const expected = 'value';
    const deep = {
      kv: {
        Value: expected,
      },
    };
    const it = _super(adapter, this.stub().returns(deep));
    it('returns string KV value when calling update/create record', function() {
      let actual = adapter.dataForRequest({
        requestType: 'updateRecord',
      });
      assert.equal(actual, expected);
      actual = adapter.dataForRequest({
        requestType: 'createRecord',
      });
      assert.equal(actual, expected);
    });
    it('returns string KV object when calling queryRecord (or anthing else) record', function() {
      const actual = adapter.dataForRequest({
        requestType: 'queryRecord',
      });
      assert.deepEqual(actual, deep);
    });
  });
  test('methodForRequest returns the correct method', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    [
      {
        request: 'deleteRecord',
        expected: 'DELETE',
      },
      {
        request: 'createRecord',
        expected: 'PUT',
      },
      {
        request: 'anythingElse',
        expected: 'GET',
      },
    ].forEach(function(item) {
      const actual = adapter.methodForRequest({ requestType: item.request });
      assert.equal(actual, item.expected);
    });
  });
});
