import config from '../config/environment';

const enabled = 'CONSUL_UI_DISABLE_REALTIME';
export function initialize(container) {
  if (config[enabled] || window.localStorage.getItem(enabled) !== null) {
    return;
  }
  ['node', 'service']
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
    .concat([
      // These are the routes where we overwrite the 'default'
      // repo service. Default repos are repos that return a promise resovlving to
      // an ember-data record or recordset
      {
        route: 'dc/nodes/index',
        services: {
          repo: 'repository/node/event-source',
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
