import { helper } from '@ember/component/helper';

export default helper(function serviceIntentionPermissions([params] /*, hash*/) {
  const hasPermissions = params.Intention.HasPermissions;
  const allowed = params.Intention.Allowed;
  const notExplicitlyDefined = params.Source === 'specific-intention' && !params.TransparentProxy;

  switch (true) {
    case hasPermissions:
      return 'allow';
    case !allowed && !hasPermissions:
      return 'deny';
    case allowed && notExplicitlyDefined:
      return 'not-defined';
    default:
      return 'allow';
  }
});
