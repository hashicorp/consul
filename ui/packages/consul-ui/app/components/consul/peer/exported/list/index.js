/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Single sortable column, matching Figma (search-only toolbar plus
// click-to-sort on the "Service name" header; no separate quick-sort dropdown
// or per-row actions).
const COLUMNS = [
  {
    label: 'Service name',
    sortKey: 'Name',
    sortValue: (item) => (item.Name || '').toLowerCase(),
    width: '100%',
  },
];

/**
 * Consul::Peer::Exported::List
 *
 * Peer-exported-services-tab specific configuration for the generic
 * Consul::DataTable. Rows link through to the service's show page (in the
 * peer's own partition/namespace when set); there are no per-row actions.
 */
export default class ConsulPeerExportedList extends Component {
  columns = COLUMNS;
}
