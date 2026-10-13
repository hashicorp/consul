/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Session lock behaviours, surfaced as the single multi-select filter group.
// Labels are capitalised for display; the values match the raw `Behavior`
// attribute on the session model.
const BEHAVIORS = [
  { value: 'release', label: 'Release' },
  { value: 'delete', label: 'Delete' },
];

/**
 * Consul::LockSession::Toolbar
 *
 * Lock-sessions-tab specific configuration for the generic `Consul::ListToolbar`.
 * It supplies the concrete filter groups (just Behaviour) but owns no Filter Bar
 * wiring itself — that all lives in the generic toolbar, including the free-text
 * search and the "Search across" dropdown.
 *
 * Unlike `Consul::HealthCheck::Toolbar`, this tab has no sort dropdown: sorting
 * is driven by the sortable column headers of `Consul::LockSession::Table`
 * (via the generic `Consul::DataTable`), matching the Figma design.
 */
export default class ConsulLockSessionToolbar extends Component {
  filterGroups = [
    {
      key: 'behavior',
      text: 'Behaviour',
      options: BEHAVIORS,
    },
  ];
}
