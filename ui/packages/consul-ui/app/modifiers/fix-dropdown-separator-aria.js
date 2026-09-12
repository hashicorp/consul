/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

export default modifier(function fixDropdownSeparatorAria(element) {
  function fixAria() {
    // HDS's <Hds::Dropdown> renders a <dd.Separator /> as
    // <li role="separator" aria-hidden="true" class="hds-dropdown-list-item
    // hds-dropdown-list-item--variant-separator"> inside a <ul>. Per the
    // ARIA-in-HTML spec's per-element role table (w3.org/TR/html-aria/
    // #docconformance), an <li> whose parent has an (implicit or explicit)
    // `list` role — true here, its parent is a plain <ul> — allows "no role
    // other than listitem", and separately the spec discourages authors from
    // ever explicitly writing the `generic` role. So neither `role=
    // "separator"` nor swapping in `role="presentation"`/`"generic"` lands in
    // the actually-permitted set for this element; accessibility checkers
    // (e.g. IBM Equal Access) flag any of them. The only option the spec
    // doesn't hedge on is no role attribute at all, falling back to the
    // element's correct implicit "listitem" role — so just remove it. The
    // <li> is already aria-hidden, so no assistive tech reads its role
    // either way; this only changes what the checker sees.
    //
    // Attached once, high up the tree (see Consul::HashicorpConsul), rather
    // than at every individual `<Hds::Dropdown>` call site, since the
    // separator <li> only exists in the DOM once a given dropdown's menu has
    // been opened at least once.
    element.querySelectorAll('li[role="separator"][aria-hidden="true"]').forEach((li) => {
      li.removeAttribute('role');
    });
  }

  setTimeout(fixAria, 100);
  const observer = new MutationObserver(fixAria);
  observer.observe(element, { childList: true, subtree: true });

  return () => observer.disconnect();
});
