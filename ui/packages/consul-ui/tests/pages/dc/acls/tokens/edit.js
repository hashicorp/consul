/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (
  visitable,
  submitable,
  deletable,
  cancelable,
  clickable,
  policySelector,
  roleSelector
) {
  return {
    visit: visitable(['/:dc/acls/tokens/:token', '/:dc/acls/tokens/create']),
    ...submitable({}, 'main form > div'),
    ...cancelable({}, 'main form > div'),
    ...deletable({}, 'main form > div'),
    use: clickable('[data-test-use]'),
    confirmUse: clickable('[data-test-confirm-use]'),
    clone: clickable('[data-test-clone]'),
    policies: policySelector(),
    roles: roleSelector(),
  };
}
