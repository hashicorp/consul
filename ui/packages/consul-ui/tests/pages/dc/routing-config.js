/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { text } from 'ember-cli-page-object';

export default function (visitable, isPresent) {
  return {
    visit: visitable('/:dc/routing-config/:name'),
    source: text('[data-test-consul-source]'),
  };
}
