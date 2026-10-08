/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

// Locality ("Source") filter options. Mirrors the legacy auth-method search bar
// locality filter and the `filter/predicates/auth-method` source predicate
// values / their translations.
const LOCALITY_OPTIONS = [
  { value: 'local', label: 'Creates local tokens' },
  { value: 'global', label: 'Creates global tokens' },
];

/**
 * Consul::AuthMethod::Toolbar
 *
 * Auth-methods-index specific configuration for the generic
 * `Consul::ListToolbar`. It supplies the Type (kind) and Source (locality)
 * filter groups and the Name / Max TTL sort dropdown; all Filter Bar wiring,
 * the free-text search and the "Search across" dropdown live in the generic
 * toolbar. The OIDC kind option is only offered when SSO is enabled, matching
 * the legacy search bar's `CONSUL_SSO_ENABLED` gate.
 */
export default class ConsulAuthMethodToolbar extends Component {
  @service('env') env;

  get filterGroups() {
    const kindOptions = [
      { value: 'kubernetes', label: 'Kubernetes' },
      { value: 'jwt', label: 'JWT' },
    ];
    if (this.env.var('CONSUL_SSO_ENABLED')) {
      kindOptions.push({ value: 'oidc', label: 'OIDC' });
    }
    return [
      { key: 'kind', text: 'Type', options: kindOptions },
      { key: 'source', text: 'Source', options: LOCALITY_OPTIONS },
    ];
  }
}
