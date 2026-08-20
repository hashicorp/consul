/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from 'consul-ui/routing/route';

export default class DcServicesIndexRoute extends Route {
  // Ember query params are "sticky" by default: their last-used values are
  // restored whenever the route is re-entered. For the services index that
  // meant a filter the user applied earlier (e.g. `status=passing`) would be
  // re-applied automatically on every later visit, making it look like a
  // default filter that could not be removed.
  //
  // Reset the filter/search/sort query params when leaving the route so that
  // navigating back to the services index always starts unfiltered. Filters
  // are then only ever applied when the user explicitly applies them.
  resetController(controller, isExiting) {
    super.resetController(...arguments);
    if (isExiting) {
      controller.setProperties({
        sortBy: undefined,
        status: undefined,
        source: undefined,
        kind: undefined,
        searchproperty: undefined,
        search: undefined,
      });
    }
  }
}
