/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (
  visitable,
  submitable,
  deletable,
  cancelable,
  policySelector,
  roleSelector
) {
  return {
    visit: visitable(['/:dc/namespaces/:namespace', '/:dc/namespaces/create']),
    ...submitable({}, 'main form > div'),
    ...cancelable({}, 'main form > div'),
    ...deletable({}, 'main form > div'),
    policies: policySelector(),
    roles: roleSelector(),
  };
}
