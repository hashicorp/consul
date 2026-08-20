/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

/**
 * Consul::Partition::Toolbar
 *
 * Admin-partitions-index specific configuration for the generic
 * Consul::ListToolbar. Admin partitions have no categorical filter groups
 * (only free-text search + a sort dropdown), so filterGroups is empty.
 * The sort dropdown ("A to Z" / "Z to A") is provided via the :quickFilters
 * slot, matching the pattern used on the intentions index page.
 */
export default class ConsulPartitionToolbar extends Component {
  // Admin partitions have no categorical filter options; only free-text
  // search (built into ListToolbar) and the sort quick-filter below.
  filterGroups = [];
}
