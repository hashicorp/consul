/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { clickable } from 'ember-cli-page-object';

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
    confirmDelete: clickable("[data-test-id='confirm-action']", {
      resetScope: true,
      testContainer: 'body',
    }),
    policies: policySelector(),
    roles: roleSelector(),
  };
}
