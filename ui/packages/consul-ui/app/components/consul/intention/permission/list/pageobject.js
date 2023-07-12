import { collection } from 'ember-cli-page-object';

export default (scope = '.consul-intention-permission-list') => {
  return {
    scope: scope,
    intentionPermissions: collection('[data-test-list-row]', {}),
  };
};
