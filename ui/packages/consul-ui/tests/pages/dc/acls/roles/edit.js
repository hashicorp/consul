/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, submitable, deletable, cancelable, policySelector, tokenList) {
  return {
    visit: visitable(['/:dc/acls/roles/:role', '/:dc/acls/roles/create']),
    ...submitable({}, 'main form .consul-role-form__actions'),
    ...cancelable({}, 'main form .consul-role-form__actions'),
    ...deletable({}, 'main form .consul-role-form__actions'),
    policies: policySelector(''),
    tokens: tokenList(),
  };
}
