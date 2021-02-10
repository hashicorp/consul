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
        instance: ['registered'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = [items[1]];
    actual = items.filter(
      predicate({
        instance: ['not-registered'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = items;
    actual = items.filter(
      predicate({
        instance: ['registered', 'not-registered'],
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
        status: ['passing'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = [items[1]];
    actual = items.filter(
      predicate({
        status: ['warning'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = items;
    actual = items.filter(
      predicate({
        status: ['passing', 'warning', 'critical'],
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
        kind: ['ingress-gateway'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = [items[1]];
    actual = items.filter(
      predicate({
        kind: ['mesh-gateway'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = items;
    actual = items.filter(
      predicate({
        kind: ['ingress-gateway', 'mesh-gateway', 'service'],
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
        kind: ['ingress-gateway'],
        status: ['passing'],
        instance: ['registered'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = [items[1]];
    actual = items.filter(
      predicate({
        kind: ['mesh-gateway'],
        status: ['warning'],
        instance: ['registered'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = items;
    actual = items.filter(
      predicate({
        kind: ['ingress-gateway', 'mesh-gateway', 'service'],
        status: ['passing', 'warning', 'critical'],
        instance: ['registered', 'not-registered'],
      })
    );
    assert.deepEqual(actual, expected);
  });
});
