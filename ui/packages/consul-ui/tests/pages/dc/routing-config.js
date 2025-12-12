/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { text } from 'ember-cli-page-object';

export default function (visitable, isPresent) {
  return {
    visit: visitable('/:dc/routing-config/:name'),
    source: text('[data-test-consul-source]'),
  };
}
