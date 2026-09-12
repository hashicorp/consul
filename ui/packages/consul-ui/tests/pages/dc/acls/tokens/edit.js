/**
 * Copyright IBM Corp. 2024, 2026
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
    // The token form uses a two-step HDS modal delete flow:
    //   delete        → [data-test-delete]         opens the confirmation modal
    //   confirmDelete → [data-test-confirm-delete]  confirms inside the modal
    ...deletable({}, 'main form > div'),
    confirmDelete: clickable('[data-test-confirm-delete]', { testContainer: '#ember-testing' }),
    use: clickable('[data-test-use]'),
    confirmUse: clickable('[data-test-confirm-use]'),
    clone: clickable('[data-test-clone]'),
    policies: policySelector(),
    roles: roleSelector(),
  };
}
