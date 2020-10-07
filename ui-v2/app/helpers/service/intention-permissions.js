import { helper } from '@ember/component/helper';

export default helper(function serviceIntentionPermissions([params] /*, hash*/) {
  const L7Permissions = params.Intention.HasL7Permissions;
  const permission = params.Intention.Allowed;

  switch (true) {
    case L7Permissions:
      return 'allow';
    case !permission && !L7Permissions:
      return 'deny';
    default:
      return 'allow';
  }
});
