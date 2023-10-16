/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (
  visitable,
  clickable,
  text,
  attribute,
  present,
  collection,
  popoverSelect
) {
  const service = {
    name: text('[data-test-service-name]'),
    service: clickable('a'),
    externalSource: attribute('data-test-external-source', '[data-test-external-source]'),
    kind: attribute('data-test-kind', '[data-test-kind]'),
    peer: text('[data-test-bucket-item="peer"]'),
    mesh: present('[data-test-mesh]'),
    associatedServiceCount: present('[data-test-associated-service-count]'),
  };
  return {
    visit: visitable('/:dc/services'),
    services: collection('.consul-service-list > ul > li:not(:first-child)', service),
    home: clickable('[data-test-home]'),
    sort: popoverSelect('[data-test-sort-control]'),
  };
}
