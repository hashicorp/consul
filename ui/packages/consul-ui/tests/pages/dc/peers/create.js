/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, submitable, clickable) {
  return {
    visit: visitable('/:dc/peers/create'),
    // Scoped: the app shell has its own unrelated [type=submit] element
    // (e.g. datacenter/nspace search), so a bare selector matches more
    // than one node.
    ...submitable({}, '.consul-peer-form'),
    tabs: {
      generate: clickable('[data-test-tab="generate"] button'),
      initiate: clickable('[data-test-tab="initiate"] button'),
    },
    cancel: clickable('[data-test-cancel]'),
    confirmDiscard: clickable('[data-test-confirm-discard]'),
    keepEditing: clickable('[data-test-keep-editing]'),
    closeTokenModal: clickable('#copy-token-modal .hds-modal__dismiss'),
  };
}
