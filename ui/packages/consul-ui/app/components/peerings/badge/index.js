/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Icon/color per the Peers list Figma design (node 3570:74800). PENDING and
// ESTABLISHING share the same treatment as the legacy icon set did. TERMINATED
// calls for a dark/high-contrast badge that isn't one of Hds::Badge's stock
// colors, so it's approximated with a `dark` flag the template turns into a
// scoped CSS override on top of `color=neutral` (see index.scss).
const BADGE_LOOKUP = {
  ACTIVE: {
    color: 'success',
    icon: 'check-circle-fill',
    tooltip: 'This peer connection is currently active.',
  },
  PENDING: {
    color: 'warning',
    icon: 'loading-static',
    tooltip: 'This peering connection has not been established yet.',
  },
  ESTABLISHING: {
    color: 'warning',
    icon: 'loading-static',
    tooltip: 'This peering connection is in the process of being established.',
  },
  FAILING: {
    color: 'critical',
    icon: 'x-circle-fill',
    tooltip:
      'This peering connection has some intermittent errors (usually network related). It will continue to retry. ',
  },
  DELETING: {
    color: 'neutral',
    icon: 'loading-static',
    tooltip: 'This peer is in the process of being deleted.',
  },
  TERMINATED: {
    color: 'neutral',
    icon: 'x-square-fill',
    dark: true,
    tooltip: 'Someone in the other peer may have deleted this peering connection.',
  },
  UNDEFINED: {
    color: 'neutral',
    icon: 'help',
    tooltip: 'The state of this peering connection is undefined.',
  },
};
export default class PeeringsBadge extends Component {
  get styles() {
    const {
      peering: { State },
    } = this.args;

    // Fall back to the UNDEFINED treatment for any state we don't recognize,
    // rather than throwing — the visible label still reflects the real value.
    return BADGE_LOOKUP[State] || BADGE_LOOKUP.UNDEFINED;
  }

  get tooltip() {
    return this.styles.tooltip;
  }

  get isDark() {
    return Boolean(this.styles.dark);
  }
}
