/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable) {
  return {
    visit: visitable('/:dc/peers/:peer'),
    visitExported: visitable('/:dc/peers/:peer/exported-services'),
    visitImported: visitable('/:dc/peers/:peer/imported-services'),
    visitAddresses: visitable('/:dc/peers/:peer/addresses'),
  };
}
