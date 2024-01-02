/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, submitable, deletable, cancelable, policySelector, tokenList) {
  return {
    visit: visitable(['/:dc/acls/roles/:role', '/:dc/acls/roles/create']),
    ...submitable({}, 'main form > div'),
    ...cancelable({}, 'main form > div'),
    ...deletable({}, 'main form > div'),
    policies: policySelector(''),
    tokens: tokenList(),
  };
}
