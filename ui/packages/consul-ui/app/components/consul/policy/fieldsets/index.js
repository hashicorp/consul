/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { set } from '@ember/object';

/**
 * Consul::Policy::Fieldsets
 *
 * Owns the `isScoped` toggle state that controls whether the datacenter
 * checkbox list is shown, and the `datacenters` list loaded via DataSource.
 * All field-change events are forwarded to the parent form via `@onChange`.
 */
export default class ConsulPolicyFieldsets extends Component {
  // true when the user has scoped the policy to specific datacenters
  @tracked isScoped = (this.args.item?.Datacenters?.length ?? 0) > 0;

  // datacenter list populated by <DataSource>
  @tracked datacenters = [];

  // saved datacenter selection while toggled to "All datacenters"
  previousDatacenters = null;

  @action
  setDatacenters(e) {
    this.datacenters = e?.data ?? [];
  }

  @action
  handleChange(e, value) {
    const name = e?.target?.name ?? '';

    if (name === 'policy[isScoped]') {
      if (this.isScoped) {
        // switching to "All datacenters" — save and clear the selection
        this.previousDatacenters = this.args.item.Datacenters;
        set(this.args.item, 'Datacenters', null);
      } else {
        // switching back to scoped — restore previous selection
        set(this.args.item, 'Datacenters', this.previousDatacenters);
        this.previousDatacenters = null;
      }
      this.isScoped = !this.isScoped;
      // notify parent that the form data has changed
      this.args.onChange?.(e, value);
      return;
    }

    this.args.onChange?.(e, value);
  }
}
