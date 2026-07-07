/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// "Type" filter options. Mirrors the legacy KV search bar's `kind` filter and
// the filter/predicates/kv folder|key predicate values.
const KIND_OPTIONS = [
  { value: 'folder', label: 'Folder' },
  { value: 'key', label: 'Key' },
];

/**
 * Consul::Kv::Toolbar
 *
 * KV-index specific configuration for the generic `Consul::ListToolbar`. It
 * supplies the Type (folder/key) filter group and the Name / Type sort
 * dropdown; all Filter Bar wiring, the free-text search and the "Search across"
 * dropdown live in the generic toolbar. Mirrors the sort options of the legacy
 * KV search bar so the existing sort behaviour is preserved.
 */
export default class ConsulKvToolbar extends Component {
  get filterGroups() {
    return [{ key: 'kind', text: 'Type', options: KIND_OPTIONS }];
  }
}
