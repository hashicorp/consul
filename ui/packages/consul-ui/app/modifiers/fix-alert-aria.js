/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

export default modifier(function fixAlertAria(element) {
  function fixAria() {
    // HDS's <Hds::Alert> only gives its root <div> a role — "alert" or
    // "alertdialog" — for the "warning"/"critical"/"success" colors (what
    // its own source calls an alert that "actually alerts users"). For
    // every other color ("neutral", "highlight", …, and the default when
    // @color is omitted, which is most of Consul's informational alerts —
    // see e.g. the Intentions "Managed by CRD" notice), no role is set at
    // all, so the <div> falls back to its implicit "generic" role.
    //
    // But HDS sets aria-labelledby (pointing at the alert's Title/
    // Description) unconditionally, regardless of that role — its own
    // source comment says the labelling is there because "`alertdialog`
    // must have an accessible name", i.e. it was only ever meant for the
    // semantic-alert branch. On a role="generic" element, aria-labelledby
    // isn't a permitted attribute at all (generic explicitly doesn't
    // support naming), so every non-semantic-colored alert with a title
    // trips this. There's nothing to preserve here for AT users — the
    // title text is already read as ordinary page content either way — so
    // just drop the attribute HDS shouldn't have set in this case.
    element.querySelectorAll('.hds-alert[aria-labelledby]:not([role])').forEach((alert) => {
      alert.removeAttribute('aria-labelledby');
    });
  }

  setTimeout(fixAria, 100);
  const observer = new MutationObserver(fixAria);
  observer.observe(element, { childList: true, subtree: true });

  return () => observer.disconnect();
});
