/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

// Column definitions for the service-instance Upstreams table. Only the first
// four columns are sortable (matches the Figma design — Address/Mode have no
// sort affordance since they're free-form/derived values). Each sortValue
// mirrors the fallback logic the old Consul::UpstreamInstance::List used when
// deciding what to display, so sorting agrees with what's on screen.
const COLUMNS = [
  {
    label: 'Upstream name',
    sortKey: 'name',
    sortValue: (item) => (item.DestinationName || '').toLowerCase(),
  },
  {
    label: 'Namespace',
    sortKey: 'namespace',
    sortValue: (item) => (item.DestinationNamespace || '').toLowerCase(),
  },
  {
    label: 'Admin partition',
    sortKey: 'partition',
    sortValue: (item) => (item.DestinationPartition || '').toLowerCase(),
  },
  {
    label: 'Datacenter',
    sortKey: 'datacenter',
    sortValue: (item) => (item.Datacenter || '').toLowerCase(),
  },
  {
    label: 'Address',
  },
  {
    label: 'Mode',
  },
];

/**
 * Consul::UpstreamInstance::Table
 *
 * Service-instance Upstreams-tab specific configuration for the generic
 * Consul::DataTable. Supplies the concrete columns (and their sort
 * comparators) and renders each row's cells via the :row block; owns no
 * sorting/pagination state itself — that lives in the generic table.
 *
 * Namespace/Admin partition fall back to the current @nspace/@partition when
 * an item doesn't specify its own (mirrors the old Consul::Bucket::List
 * fallback), and render "-" for prepared_query destinations, which have
 * neither.
 *
 * The Namespace/Admin partition columns are themselves omitted entirely
 * (header and cells) when nspaces/partitions aren't supported — e.g. CE/OSS
 * builds — mirroring the `abilities.can('use nspaces' | 'use partitions')`
 * checks Consul::Bucket::List already uses for the same fields elsewhere.
 *
 * @argument {Array} items - the upstream instances to display.
 * @argument {string} dc - the current datacenter, used to blank the
 *   Datacenter column when an item's datacenter matches it.
 * @argument {string} nspace - the current namespace, used as the Namespace
 *   column fallback.
 * @argument {string} partition - the current partition, used as the Admin
 *   partition column fallback.
 */
export default class ConsulUpstreamInstanceTable extends Component {
  @service abilities;

  get showNamespace() {
    return this.abilities.can('use nspaces');
  }

  get showPartition() {
    return this.abilities.can('use partitions');
  }

  get columns() {
    return COLUMNS.filter((column) => {
      if (column.sortKey === 'namespace') {
        return this.showNamespace;
      }
      if (column.sortKey === 'partition') {
        return this.showPartition;
      }
      return true;
    });
  }

  get dc() {
    return this.args.dc;
  }

  get nspace() {
    return this.args.nspace;
  }

  get partition() {
    return this.args.partition;
  }

  isPreparedQuery = (item) => item.DestinationType === 'prepared_query';

  namespaceFor = (item) => {
    if (this.isPreparedQuery(item)) {
      return undefined;
    }
    return item.DestinationNamespace || this.nspace;
  };

  partitionFor = (item) => {
    if (this.isPreparedQuery(item)) {
      return undefined;
    }
    return item.DestinationPartition || this.partition;
  };

  datacenterFor = (item) => {
    if (!item.Datacenter || item.Datacenter === this.dc) {
      return undefined;
    }
    return item.Datacenter;
  };

  // Local bind socket path takes precedence over address:port, matching the
  // old list's {{#if item.LocalBindSocketPath}}...{{else}}... branching.
  addressFor = (item) => {
    if (item.LocalBindSocketPath) {
      return item.LocalBindSocketPath;
    }
    if (item.LocalBindPort > 0) {
      return `${item.LocalBindAddress || '127.0.0.1'}:${item.LocalBindPort}`;
    }
    return undefined;
  };

  modeFor = (item) => {
    if (item.LocalBindSocketPath) {
      return item.LocalBindSocketMode || undefined;
    }
    return undefined;
  };
}
