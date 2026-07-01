/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (
  visitable,
  clickable,
  text,
  attribute,
  present,
  collection
) {
  const service = {
    name: text('[data-test-service-name]'),
    service: clickable('a'),
    externalSource: attribute('data-test-external-source', '[data-test-external-source]'),
    kind: text('.consul-service-table__type strong'),
    mesh: present('[data-test-mesh]'),
    associatedServiceCount: present('[data-test-associated-service-count]'),
  };
  return {
    visit: visitable('/:dc/services'),
    services: collection('.consul-service-table tbody tr', service),
    home: clickable('[data-test-home]'),
    sort: {
      name: clickable(
        '.consul-service-table thead th:nth-child(1) button.hds-table__th-button--sort'
      ),
      health: clickable(
        '.consul-service-table thead th:nth-child(2) button.hds-table__th-button--sort'
      ),
    },
  };
}
