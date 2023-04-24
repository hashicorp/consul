import { collection } from 'ember-cli-page-object';

export default (scope = '.consul-intention-permission-header-list') => {
  return {
    scope: scope,
    intentionPermissionHeaders: collection('[data-test-list-row]', {}),
  };
};
