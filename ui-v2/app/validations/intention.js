import { validatePresence, validateLength } from 'ember-changeset-validations/validators';
import { env } from 'consul-ui/env';
export default Object.assign(
  {
    SourceName: [validatePresence(true), validateLength({ min: 1 })],
    DestinationName: [validatePresence(true), validateLength({ min: 1 })],
    Action: validatePresence(true),
  },
  env('CONSUL_NSPACES_ENABLED')
    ? {
        SourceNS: [validatePresence(true), validateLength({ min: 1 })],
        DestinationNS: [validatePresence(true), validateLength({ min: 1 })],
      }
    : {}
);
