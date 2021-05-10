export default function(type) {
  return function(cb, withNspaces, withoutNspaces, container, assert) {
    let CONSUL_NSPACES_ENABLED = true;
    const env = container.owner.lookup('service:env');
    env.var = function() {
      return CONSUL_NSPACES_ENABLED;
    };
    const adapter = container.owner.lookup(`adapter:${type}`);
    const serializer = container.owner.lookup(`serializer:${type}`);
    const client = container.owner.lookup('service:client/http');
    let actual;

    actual = cb(adapter, serializer, client);
    assert.deepEqual(actual[0], withNspaces);

    CONSUL_NSPACES_ENABLED = false;
    actual = cb(adapter, serializer, client);
    assert.deepEqual(actual[0], withoutNspaces);
  };
}
