/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';

export default helper(function serviceCardPermissions([params] /*, hash*/) {
  if (params.Datacenter === '') {
    return 'empty';
  } else {
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
  }
});
