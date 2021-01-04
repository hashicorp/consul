import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import getNspaceRunner from 'consul-ui/tests/helpers/get-nspace-runner';

const nspaceRunner = getNspaceRunner('intention');

module('Integration | Adapter | intention', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'SourceNS:SourceName:DestinationNS:DestinationName';
  test('requestForQuery returns the correct url', function(assert) {
    return nspaceRunner(
      (adapter, serializer, client) => {
        return adapter.requestForQuery(client.body, {
          dc: dc,
          ns: 'team-1',
          filter: '*',
          index: 1,
        });
      },
      {
        filter: '*',
        index: 1,
        ns: '*',
      },
      {
        filter: '*',
        index: 1,
      },
      this,
      assert
    );
  });
  test('requestForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/connect/intentions/exact?source=SourceNS%2FSourceName&destination=DestinationNS%2FDestinationName&dc=${dc}`;
    const actual = adapter
      .requestForQueryRecord(client.url, {
        dc: dc,
        id: id,
      })
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
  test('requestForCreateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/connect/intentions/exact?source=SourceNS%2FSourceName&destination=DestinationNS%2FDestinationName&dc=${dc}`;
    const actual = adapter
      .requestForCreateRecord(
        client.url,
        {},
        {
          Datacenter: dc,
          SourceNS: 'SourceNS',
          SourceName: 'SourceName',
          DestinationNS: 'DestinationNS',
          DestinationName: 'DestinationName',
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForUpdateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/connect/intentions/exact?source=SourceNS%2FSourceName&destination=DestinationNS%2FDestinationName&dc=${dc}`;
    const actual = adapter
      .requestForUpdateRecord(
        client.url,
        {},
        {
          Datacenter: dc,
          SourceNS: 'SourceNS',
          SourceName: 'SourceName',
          DestinationNS: 'DestinationNS',
          DestinationName: 'DestinationName',
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForDeleteRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const client = this.owner.lookup('service:client/http');
    const expected = `DELETE /v1/connect/intentions/exact?source=SourceNS%2FSourceName&destination=DestinationNS%2FDestinationName&dc=${dc}`;
    const actual = adapter
      .requestForDeleteRecord(
        client.url,
        {},
        {
          Datacenter: dc,
          SourceNS: 'SourceNS',
          SourceName: 'SourceName',
          DestinationNS: 'DestinationNS',
          DestinationName: 'DestinationName',
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
});
