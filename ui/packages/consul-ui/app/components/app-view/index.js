/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class AppViewComponent extends Component {
  @service('breadcrumbs') breadcrumbs;
  @service('router') router;
  @service('routlet') routlet;

  // Tracked so that the computedBreadcrumbs getter re-evaluates after each
  // route transition.  Ember's router service does not expose currentRouteName
  // as a @tracked property, so we mirror it here via routeDidChange.
  @tracked _currentRouteName = this.router.currentRouteName;

  constructor() {
    super(...arguments);
    this.router.on('routeDidChange', this._onRouteDidChange);
  }

  willDestroy() {
    super.willDestroy(...arguments);
    this.router.off('routeDidChange', this._onRouteDidChange);
  }

  /**
   * Returns the breadcrumb item array for the current route, computed from the
   * `BreadcrumbsService`.  Used by the template when no explicit
   * `<:breadcrumbs>` named block is provided by the caller.
   *
   * @returns {BreadcrumbItem[]}
   */
  get computedBreadcrumbs() {
    const routeName = this._currentRouteName;
    if (!routeName || !this.breadcrumbs.shouldShowBreadcrumbs(routeName)) {
      return [];
    }
    const params = this.routlet.paramsFor(routeName);
    return this.breadcrumbs.computeBreadcrumbs(routeName, params);
  }

  @action
  _onRouteDidChange() {
    this._currentRouteName = this.router.currentRouteName;
  }
}
