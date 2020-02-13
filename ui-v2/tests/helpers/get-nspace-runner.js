import Service from '@ember/service';
export default function(type) {
  return function(cb, withNspaces, withoutNspaces, container, assert) {
    let CONSUL_NSPACES_ENABLED = true;
    container.owner.register(
      'service:env',
      Service.extend({
        env: function() {
          return CONSUL_NSPACES_ENABLED;
        },
      })
    );
    const adapter = container.owner.lookup(`adapter:${type}`);
    const serializer = container.owner.lookup(`serializer:${type}`);
    const client = container.owner.lookup('service:client/http');

    assert.deepEqual(cb(adapter, serializer, client), withNspaces);

    CONSUL_NSPACES_ENABLED = false;
    assert.deepEqual(cb(adapter, serializer, client), withoutNspaces);
  };
}
