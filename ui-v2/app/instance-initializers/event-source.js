import { env } from 'consul-ui/env';

export function initialize(container) {
  if (env('CONSUL_UI_DISABLE_REALTIME')) {
    return;
  }
  []
    .concat(
      ['policy', 'role'].map(function(item) {
        // create repositories that return a promise resolving to an EventSource
        return {
          service: `repository/${item}/component`,
          extend: 'repository/type/component',
          // Inject our original respository that is used by this class
          // within the callable of the EventSource
          services: {
            content: `repository/${item}`,
          },
        };
      })
    )
    .concat([
      {
        service: 'form',
        services: {
          role: 'repository/role/component',
          policy: 'repository/policy/component',
        },
      },
    ])
    .forEach(function(definition) {
      if (typeof definition.extend !== 'undefined') {
        // Create the class instances that we need
        container.register(
          `service:${definition.service}`,
          container.resolveRegistration(`service:${definition.extend}`).extend({})
        );
      }
      Object.keys(definition.services).forEach(function(name) {
        const servicePath = definition.services[name];
        // inject its dependencies, this could probably detect the type
        // but hardcode this for the moment
        if (typeof definition.route !== 'undefined') {
          container.inject(`route:${definition.route}`, name, `service:${servicePath}`);
        } else {
          container.inject(`service:${definition.service}`, name, `service:${servicePath}`);
        }
      });
    });
}

export default {
  initialize,
};
