/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, text) =>
  (scope = '.consul-health-check-list') => {
    return collection(`${scope} li`, {
      name: text('.health-check-output__name'),
      type: text('[data-health-check-type]'),
      exposed: text('[data-test-exposed]'),
    });
  };
