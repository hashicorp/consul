/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import matchesRoute from 'consul-ui/utils/routing/matches-route';
import routeInfoEqual from 'consul-ui/utils/routing/route-info-equal';

/**
 * Decides whether Hds::AppHeader's NavigationNarrator (ember-a11y-refocus)
 * should move focus to the top of the page after a route change.
 */
export default class NavigationFocusService extends Service {
  tabRoute;

  /**
   * Called by TabNav when one of its tabs starts a route transition. Per the
   * WAI-ARIA tabs pattern focus stays on the activated tab, so the resulting
   * route change must not move focus away from it.
   */
  tabTransitionTo(route) {
    this.tabRoute = route;
  }

  routeChangeValidator = (transition) => {
    const { tabRoute } = this;
    this.tabRoute = undefined;
    if (tabRoute && matchesRoute(transition.to?.name, tabRoute)) {
      return false;
    }
    return !routeInfoEqual(transition.from, transition.to);
  };
}
