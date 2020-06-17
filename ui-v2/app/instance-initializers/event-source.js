import { env } from 'consul-ui/env';

export function initialize(container) {
  if (env('CONSUL_UI_DISABLE_REALTIME')) {
    return;
  }
  ['node', 'coordinate', 'session', 'service', 'proxy', 'discovery-chain', 'intention']
    .concat(env('CONSUL_NSPACES_ENABLED') ? ['nspace/enabled'] : [])
    .map(function(item) {
      // create repositories that return a promise resolving to an EventSource
      return {
        service: `repository/${item}/event-source`,
        extend: 'repository/type/event-source',
        // Inject our original respository that is used by this class
        // within the callable of the EventSource
        services: {
          content: `repository/${item}`,
        },
      };
    })
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
      // These are the routes where we overwrite the 'default'
      // repo service. Default repos are repos that return a promise resolving to
      // an ember-data record or recordset
      {
        route: 'dc/nodes/index',
        services: {
          repo: 'repository/node/event-source',
        },
      },
      {
        route: 'dc/nodes/show',
        services: {
          repo: 'repository/node/event-source',
          coordinateRepo: 'repository/coordinate/event-source',
          sessionRepo: 'repository/session/event-source',
        },
      },
      {
        route: 'dc/services/index',
        services: {
          repo: 'repository/service/event-source',
        },
      },
      {
        route: 'dc/services/show',
        services: {
          repo: 'repository/service/event-source',
          chainRepo: 'repository/discovery-chain/event-source',
          intentionRepo: 'repository/intention/event-source',
        },
      },
      {
        route: 'dc/services/instance',
        services: {
          repo: 'repository/service/event-source',
          proxyRepo: 'repository/proxy/event-source',
        },
      },
      {
        route: 'dc/intentions/index',
        services: {
          repo: 'repository/intention/event-source',
        },
      },
      {
        service: 'form',
        services: {
          role: 'repository/role/component',
          policy: 'repository/policy/component',
        },
      },
    ])
    .concat(
      env('CONSUL_NSPACES_ENABLED')
        ? [
            {
              route: 'dc/nspaces/index',
              services: {
                repo: 'repository/nspace/enabled/event-source',
              },
            },
          ]
        : []
    )
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
          if (env('CONSUL_NSPACES_ENABLED') && definition.route.startsWith('dc/')) {
            container.inject(`route:nspace/${definition.route}`, name, `service:${servicePath}`);
          }
        } else {
          container.inject(`service:${definition.service}`, name, `service:${servicePath}`);
        }
      });
    });
}

export default {
  initialize,
};
