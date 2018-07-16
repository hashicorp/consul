import { module } from 'qunit';
import test from 'ember-sinon-qunit/test-support/test';
import { setupTest } from 'ember-qunit';

module('Unit | Adapter | application', function(hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function(assert) {
    const adapter = this.owner.lookup('adapter:application');
    assert.ok(adapter);
  });
  test('slugFromURL returns the slug (on the assumptions its the last chunk of the url)', function(assert) {
    const adapter = this.owner.lookup('adapter:application');
    const decode = this.stub().returnsArg(0);
    const expected = 'slug';
    const actual = adapter.slugFromURL({ pathname: `/this/is/a/url/with/a/${expected}` }, decode);
    assert.equal(actual, expected);
    assert.ok(decode.calledOnce);
  });
  test("uidForURL returns the a 'unique' hash for the uid using the entire url", function(assert) {
    const adapter = this.owner.lookup('adapter:application');
    const hash = this.stub().returnsArg(0);
    const expected = ['dc-1', 'slug'];
    const url = {
      pathname: `/this/is/a/url/with/a/${expected[1]}`,
      searchParams: {
        get: this.stub().returns('dc-1'),
      },
    };
    const actual = adapter.uidForURL(url, '', hash);
    assert.deepEqual(actual, expected);
    assert.ok(hash.calledOnce);
    assert.ok(url.searchParams.get.calledOnce);
  });
  test("uidForURL returns the a 'unique' hash for the uid when specifying the slug", function(assert) {
    const adapter = this.owner.lookup('adapter:application');
    const hash = this.stub().returnsArg(0);
    const expected = ['dc-1', 'slug'];
    const url = {
      searchParams: {
        get: this.stub().returns('dc-1'),
      },
    };
    const actual = adapter.uidForURL(url, expected[1], hash);
    assert.deepEqual(actual, expected);
    assert.ok(hash.calledOnce);
    assert.ok(url.searchParams.get.calledOnce);
  });
  test("uidForURL throws an error if it can't find a datacenter on the search params", function(assert) {
    const adapter = this.owner.lookup('adapter:application');
    const hash = this.stub().returnsArg(0);
    const expected = ['dc-1', 'slug'];
    const url = {
      pathname: `/this/is/a/url/with/a/${expected[1]}`,
      searchParams: {
        get: this.stub().returns(''),
      },
    };
    assert.throws(function() {
      adapter.uidForURL(url, expected[1], hash);
    }, /datacenter/);
    assert.ok(url.searchParams.get.calledOnce);
  });
  test("uidForURL throws an error if it can't find a slug", function(assert) {
    const adapter = this.owner.lookup('adapter:application');
    const hash = this.stub().returnsArg(0);
    const url = {
      pathname: `/`,
      searchParams: {
        get: this.stub().returns('dc-1'),
      },
    };
    assert.throws(function() {
      adapter.uidForURL(url, '', hash);
    }, /slug/);
    assert.ok(url.searchParams.get.calledOnce);
  });
  test("uidForURL throws an error if it can't find a slug", function(assert) {
    const adapter = this.owner.lookup('adapter:application');
    const hash = this.stub().returnsArg(0);
    const url = {
      pathname: `/`,
      searchParams: {
        get: this.stub().returns('dc-1'),
      },
    };
    assert.throws(function() {
      adapter.uidForURL(url, '', hash);
    }, /slug/);
    assert.ok(url.searchParams.get.calledOnce);
  });
  test('handleBooleanResponse returns the expected pojo structure', function(assert) {
    const adapter = this.owner.lookup('adapter:application');
    adapter.uidForURL = this.stub().returnsArg(0);
    const expected = {
      'primary-key-name': 'url',
    };
    const actual = adapter.handleBooleanResponse('url', {}, Object.keys(expected)[0], 'slug');
    assert.deepEqual(actual, expected);
    assert.ok(adapter.uidForURL.calledOnce);
  });
  test('handleSingleResponse returns the expected pojo structure', function(assert) {
    const adapter = this.owner.lookup('adapter:application');
    const url = {
      pathname: `/`,
      searchParams: {
        get: this.stub().returns('dc-1'),
      },
    };
    adapter.uidForURL = this.stub().returns('name');
    const expected = {
      Datacenter: 'dc-1',
      Name: 'name',
      'primary-key-name': 'name',
    };
    const actual = adapter.handleSingleResponse(url, { Name: 'name' }, 'primary-key-name', 'Name');
    assert.deepEqual(actual, expected);
    assert.ok(adapter.uidForURL.calledOnce);
  });
  test('handleBatchResponse returns the expected pojo structure', function(assert) {
    const adapter = this.owner.lookup('adapter:application');
    const url = {
      pathname: `/`,
      searchParams: {
        get: this.stub().returns('dc-1'),
      },
    };
    adapter.uidForURL = this.stub().returnsArg(1);
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
    const actual = adapter.handleBatchResponse(
      url,
      [{ Name: 'name1' }, { Name: 'name2' }],
      'primary-key-name',
      'Name'
    );
    assert.deepEqual(actual, expected);
    assert.ok(adapter.uidForURL.calledTwice);
  });
});
