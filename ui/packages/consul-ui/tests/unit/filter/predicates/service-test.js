import predicates from 'consul-ui/filter/predicates/service';
import { andOr } from 'consul-ui/utils/filter';
import { module, test } from 'qunit';

module('Unit | Filter | Predicates | service', function() {
  const predicate = andOr(predicates);

  test('it returns registered/unregistered items depending on instance count', function(assert) {
    const items = [
      {
        InstanceCount: 1,
      },
      {
        InstanceCount: 0,
      },
    ];

    let expected, actual;

    expected = [items[0]];
    actual = items.filter(
      predicate({
        instances: ['registered'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = [items[1]];
    actual = items.filter(
      predicate({
        instances: ['not-registered'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = items;
    actual = items.filter(
      predicate({
        instances: ['registered', 'not-registered'],
      })
    );
    assert.deepEqual(actual, expected);
  });

  test('it returns items depending on status', function(assert) {
    const items = [
      {
        MeshStatus: 'passing',
      },
      {
        MeshStatus: 'warning',
      },
      {
        MeshStatus: 'critical',
      },
    ];

    let expected, actual;

    expected = [items[0]];
    actual = items.filter(
      predicate({
        statuses: ['passing'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = [items[1]];
    actual = items.filter(
      predicate({
        statuses: ['warning'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = items;
    actual = items.filter(
      predicate({
        statuses: ['passing', 'warning', 'critical'],
      })
    );
    assert.deepEqual(actual, expected);
  });

  test('it returns items depending on service type', function(assert) {
    const items = [
      {
        Kind: 'ingress-gateway',
      },
      {
        Kind: 'mesh-gateway',
      },
      {},
    ];

    let expected, actual;

    expected = [items[0]];
    actual = items.filter(
      predicate({
        kinds: ['ingress-gateway'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = [items[1]];
    actual = items.filter(
      predicate({
        kinds: ['mesh-gateway'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = items;
    actual = items.filter(
      predicate({
        kinds: ['ingress-gateway', 'mesh-gateway', 'service'],
      })
    );
    assert.deepEqual(actual, expected);
  });
  test('it returns items depending on a mixture of properties', function(assert) {
    const items = [
      {
        Kind: 'ingress-gateway',
        MeshStatus: 'passing',
        InstanceCount: 1,
      },
      {
        Kind: 'mesh-gateway',
        MeshStatus: 'warning',
        InstanceCount: 1,
      },
      {
        MeshStatus: 'critical',
        InstanceCount: 0,
      },
    ];

    let expected, actual;

    expected = [items[0]];
    actual = items.filter(
      predicate({
        kinds: ['ingress-gateway'],
        statuses: ['passing'],
        instances: ['registered'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = [items[1]];
    actual = items.filter(
      predicate({
        kinds: ['mesh-gateway'],
        statuses: ['warning'],
        instances: ['registered'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = items;
    actual = items.filter(
      predicate({
        kinds: ['ingress-gateway', 'mesh-gateway', 'service'],
        statuses: ['passing', 'warning', 'critical'],
        instances: ['registered', 'not-registered'],
      })
    );
    assert.deepEqual(actual, expected);
  });
});
