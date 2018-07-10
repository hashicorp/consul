import { module, skip } from 'qunit';
import { setupTest } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import stubSuper from 'consul-ui/tests/helpers/stub-super';

module('Unit | Adapter | kv', function(hooks) {
  setupTest(hooks);

  skip('what should handleResponse return when createRecord is called with a `false` payload');

  // Replace this with your real tests.
  test('it exists', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    assert.ok(adapter);
  });
  test('handleResponse with single type requests', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const expected = 'key/name';
    const dc = 'dc1';
    const url = `/v1/kv/${expected}?dc=${dc}`;
    // handleResponse calls `urlForCreateRecord`, so stub that out
    // so we are testing a single unit of code
    const urlForCreateRecord = this.stub(adapter, 'urlForCreateRecord');
    urlForCreateRecord.returns(url);
    // handleResponse calls `this._super`, so stub that out also
    // so we only test a single unit
    // calling `it() will now create a 'sandbox' with a stubbed `_super`
    const it = stubSuper(adapter, function(status, headers, response, requestData) {
      return response;
    });

    // right now, the message here is more for documentation purposes
    // and to replicate the `test`/`it` API
    // it does not currently get printed to the QUnit test runner output

    // the following tests use our stubbed `_super` sandbox
    it('returns a KV uid pojo when createRecord is called with a `true` payload', function() {
      const uid = {
        uid: JSON.stringify([dc, expected]),
      };
      const headers = {};
      const actual = adapter.handleResponse(200, headers, true, { url: url });
      assert.deepEqual(actual, uid);
    });
    it("returns the original payload plus the uid if it's not a Boolean", function() {
      const uid = {
        uid: JSON.stringify([dc, expected]),
      };
      const actual = adapter.handleResponse(200, {}, [uid], { url: url });
      assert.deepEqual(actual, uid);
    });
  });
  skip('handleRequest for multiple type requests');
  test('dataForRequest returns', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    // dataForRequest goes through window.atob
    adapter.decoder = {
      execute: this.stub().returnsArg(0),
    };
    //
    const expected = 'value';
    const deep = {
      kv: {
        Value: expected,
      },
    };
    const it = stubSuper(adapter, this.stub().returns(deep));
    it('returns string KV value when calling update/create record', function() {
      const requests = [
        {
          request: 'updateRecord',
          expected: expected,
        },
        {
          request: 'createRecord',
          expected: expected,
        },
      ];
      requests.forEach(function(item, i, arr) {
        const actual = adapter.dataForRequest({
          requestType: item.request,
        });
        assert.equal(actual, expected);
      });
    });
    // not included in the above forEach as it's a slightly different concept
    it('returns string KV object when calling queryRecord (or anything else) record', function() {
      const actual = adapter.dataForRequest({
        requestType: 'queryRecord',
      });
      assert.equal(actual, null);
    });
  });
  test('methodForRequest returns the correct method', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const requests = [
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
    ];
    requests.forEach(function(item) {
      const actual = adapter.methodForRequest({ requestType: item.request });
      assert.equal(actual, item.expected);
    });
  });
});
