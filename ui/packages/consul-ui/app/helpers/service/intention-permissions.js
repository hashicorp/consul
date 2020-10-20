import { helper } from '@ember/component/helper';

export default helper(function serviceIntentionPermissions([params] /*, hash*/) {
  const hasPermissions = params.Intention.HasPermissions;
  const allowed = params.Intention.Allowed;

  switch (true) {
    case hasPermissions:
      return 'allow';
    case !allowed && !hasPermissions:
      return 'deny';
    default:
      return 'allow';
  }
});
