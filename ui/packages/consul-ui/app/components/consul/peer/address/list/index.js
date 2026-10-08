/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Single column, not sortable (per Figma — no sort icon on "Address"; server
// addresses have no inherent order worth exposing).
const COLUMNS = [{ label: 'Address', width: '100%' }];

/**
 * Consul::Peer::Address::List
 *
 * Server-addresses-tab specific configuration for the generic
 * Consul::DataTable. Each row is the plain address string plus a copy-to-
 * clipboard action; there is no per-row navigation.
 */
export default class ConsulPeerAddressList extends Component {
  columns = COLUMNS;
}
