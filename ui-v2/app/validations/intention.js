import { validatePresence, validateLength } from 'ember-changeset-validations/validators';
import config from 'consul-ui/config/environment';
export default Object.assign(
  {
    SourceName: [validatePresence(true), validateLength({ min: 1 })],
    DestinationName: [validatePresence(true), validateLength({ min: 1 })],
    Action: validatePresence(true),
  },
  config.CONSUL_NAMESPACES_ENABLED
    ? {
        SourceNS: [validatePresence(true), validateLength({ min: 1 })],
        DestinationNS: [validatePresence(true), validateLength({ min: 1 })],
      }
    : {}
);
