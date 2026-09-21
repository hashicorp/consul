/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

const STATUSES = ['passing', 'warning', 'critical', 'empty'];
const KINDS = ['service', 'node'];
const CHECK_TYPES = ['alias', 'docker', 'grpc', 'http', 'script', 'serf', 'tcp', 'ttl'];

// Maps a `<Field>:<direction>` sort value (as used by @sort.value) to the
// translation key for its toggle-button label. Mirrors the options offered
// in the sort dropdown below.
const SORT_LABEL_KEYS = {
  'Status:asc': 'common.sort.status.asc',
  'Status:desc': 'common.sort.status.desc',
  'Name:asc': 'common.sort.alpha.asc',
  'Name:desc': 'common.sort.alpha.desc',
  'Kind:asc': 'components.consul.health-check.search-bar.sort.kind.asc',
  'Kind:desc': 'components.consul.health-check.search-bar.sort.kind.desc',
};

/**
 * Consul::HealthCheck::Toolbar
 *
 * Health-checks-tab specific configuration for the generic `Consul::ListToolbar`,
 * used by both the node detail and the service-instance detail Health-checks
 * tabs. It supplies the concrete filter groups (Health status / Kind / Type)
 * but owns no Filter Bar wiring itself — that all lives in the generic toolbar.
 * The sort control (grouped by Health status / Check name / Check type) is
 * rendered directly in this component's `:quickFilters` block since, unlike the
 * other migrated toolbars, there are no health quick-filter buttons on this tab.
 *
 * The Kind filter is only available on pages that wire up a `kind` filter (the
 * node tab); the service-instance tab doesn't, so that group is omitted there —
 * mirroring the `{{#if @filter.kind}}` guard the old search bar used.
 */
export default class ConsulHealthCheckToolbar extends Component {
  @service intl;

  // The multi-select filter groups passed to the generic toolbar. Labels are
  // pulled from the same translation keys the old health-check search-bar
  // used, so no new copy is introduced.
  get filterGroups() {
    const groups = [
      {
        key: 'status',
        text: this.intl.t('common.consul.status'),
        options: STATUSES.map((value) => ({
          value,
          label: this.intl.t(`common.consul.${value}`),
        })),
      },
    ];

    // Only offer the Kind filter when the page provides a `kind` filter to
    // drive it (node tab). The service-instance tab has no `kind` query param.
    if (this.args.filter?.kind) {
      groups.push({
        key: 'kind',
        text: this.intl.t('components.consul.health-check.search-bar.kind.name'),
        options: KINDS.map((value) => ({
          value,
          label: this.intl.t(`components.consul.health-check.search-bar.kind.options.${value}`),
        })),
      });
    }

    groups.push({
      key: 'check',
      text: this.intl.t('components.consul.health-check.search-bar.check.name'),
      options: CHECK_TYPES.map((value) => ({
        value,
        label: this.intl.t(`components.consul.health-check.search-bar.check.options.${value}`),
      })),
    });

    return groups;
  }

  // Label shown on the sort dropdown's toggle button for the currently
  // active sort value.
  get sortLabel() {
    const key = SORT_LABEL_KEYS[this.args.sort?.value];
    return key ? this.intl.t(key) : '';
  }
}
