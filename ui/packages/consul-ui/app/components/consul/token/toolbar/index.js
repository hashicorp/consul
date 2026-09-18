/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Token "Type" filter options. Mirrors the legacy token search bar kind filter
// and the `filter/predicates/token` kind predicate values.
const KIND_OPTIONS = [
  { value: 'global-management', label: 'Global management' },
  { value: 'global', label: 'Global' },
  { value: 'local', label: 'Local' },
];

/**
 * Consul::Token::Toolbar
 *
 * Tokens-index specific configuration for the generic `Consul::ListToolbar`. It
 * supplies the concrete filter group (Type) only; all Filter Bar wiring, the
 * free-text search and the "Search across" dropdown live in the generic
 * toolbar.
 */
export default class ConsulTokenToolbar extends Component {
  get filterGroups() {
    return [{ key: 'kind', text: 'Type', options: KIND_OPTIONS }];
  }
}
