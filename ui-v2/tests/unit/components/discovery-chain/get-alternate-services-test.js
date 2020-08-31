import { getAlternateServices } from 'consul-ui/components/discovery-chain/utils';
import { module, test } from 'qunit';

module('Unit | Component | discovery-chain/get-alternative-services', function() {
  test('it guesses a different namespace', function(assert) {
    const expected = {
      Type: 'Namespace',
      Targets: ['different-ns', 'different-ns2'],
    };
    const actual = getAlternateServices(
      ['service.different-ns.dc', 'service.different-ns2.dc'],
      'service.namespace.dc'
    );
    assert.equal(actual.Type, expected.Type);
    assert.deepEqual(actual.Targets, expected.Targets);
  });
  test('it guesses a different datacenter', function(assert) {
    const expected = {
      Type: 'Datacenter',
      Targets: ['dc1', 'dc2'],
    };
    const actual = getAlternateServices(
      ['service.namespace.dc1', 'service.namespace.dc2'],
      'service.namespace.dc'
    );
    assert.equal(actual.Type, expected.Type);
    assert.deepEqual(actual.Targets, expected.Targets);
  });
  test('it guesses a different service', function(assert) {
    const expected = {
      Type: 'Service',
      Targets: ['service-2', 'service-3'],
    };
    const actual = getAlternateServices(
      ['service-2.namespace.dc', 'service-3.namespace.dc'],
      'service.namespace.dc'
    );
    assert.equal(actual.Type, expected.Type);
    assert.deepEqual(actual.Targets, expected.Targets);
  });
  test('it guesses a different subset', function(assert) {
    const expected = {
      Type: 'Subset',
      Targets: ['v3', 'v2'],
    };
    const actual = getAlternateServices(
      ['v3.service.namespace.dc', 'v2.service.namespace.dc'],
      'v1.service.namespace.dc'
    );
    assert.equal(actual.Type, expected.Type);
    assert.deepEqual(actual.Targets, expected.Targets);
  });
});
