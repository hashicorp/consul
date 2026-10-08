/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import mergeChecks from 'consul-ui/utils/merge-checks';
import { healthFromChecks } from 'consul-ui/utils/health-status';

/**
 * Consul::ServiceInstance::Table
 *
 * Service-instances specific configuration for the generic Consul::DataTable.
 * It supplies the concrete columns (and their sort comparators) and renders
 * each row's cells via the :row block, reusing the same presentational
 * sub-components (Consul::InstanceChecks, Consul::ExternalSource, TagList, ...)
 * the legacy Consul::ServiceInstance::List used so no per-instance information
 * is lost.
 *
 * The component renders in two modes, driven by `@node`:
 *  - Service > Instances tab (`@node` unset): shows the Node column and the
 *    Node health column, links each row to the instance detail page.
 *  - Node > Service Instances tab (`@node` set): hides the Node column and Node
 *    health column (you are already on the node, and node health lives on the
 *    node's own health-checks tab) and links each row to the service page.
 *
 * It performs no data fetching itself; it receives the already
 * fetched / filtered / searched `@items` from the data layer.
 *
 * @argument {Array} items - the service instances to display.
 * @argument {Array} proxies - proxy service instances from the same service,
 *   used to merge proxy checks into a row's service checks.
 * @argument {(Object|boolean)} [node] - when set, renders the node-page view.
 * @argument {string} routeName - Ember route the row name links to.
 * @argument {string} [nspace] - active namespace, for link params.
 * @argument {string} [partition] - active partition, for link params.
 */
export default class ConsulServiceInstanceTable extends Component {
  // Whether we are rendering inside a node page (hides node-specific columns).
  get isNodeView() {
    return Boolean(this.args.node);
  }

  // Map of proxy service instance keyed by the service ID it proxies, so a
  // row can find its proxy's checks. Mirrors the `to-hash` the legacy list did.
  get proxyLookup() {
    const map = {};
    (this.args.proxies || []).forEach((proxy) => {
      const id = proxy.Service?.Proxy?.DestinationServiceID;
      if (id !== undefined) {
        map[id] = proxy;
      }
    });
    return map;
  }

  // The merged (service + proxy) checks for an instance, matching the legacy
  // list's `merge-checks` call. Node checks (ServiceName === '') are
  // de-duplicated by the merge util.
  mergedChecks = (item) => {
    const proxy = this.proxyLookup[item.Service?.ID];
    const lists = [item.Checks];
    if (proxy) {
      lists.push(proxy.Checks || []);
    }
    // merge-checks mutates the array it's given via shift(); pass a copy.
    return mergeChecks([...lists]);
  };

  // Service-level checks for an instance (ServiceID !== '' filtered out by the
  // legacy list via `filter-by 'ServiceID' ''`).
  serviceChecks = (item) => {
    return this.mergedChecks(item).filter((check) => check.ServiceID === '');
  };

  // Node-level checks for an instance (the legacy list used
  // `reject-by 'ServiceID' ''`).
  nodeChecks = (item) => {
    return this.mergedChecks(item).filter((check) => check.ServiceID !== '');
  };

  // `{ status, count, total }` descriptors for the two health columns, fed
  // straight to Consul::HealthBadge (which renders them with counts here).
  serviceHealth = (item) => {
    return healthFromChecks(this.serviceChecks(item));
  };

  nodeHealth = (item) => {
    return healthFromChecks(this.nodeChecks(item));
  };

  // Column definitions, ordered to match the migration designs:
  // Instance, Service health, Service mesh, Node, Node health, Address,
  // External source. Only Instance and Node are sortable — the designs carry a
  // sort control on those two alone, and ranking rows by health sorts them by
  // an internal status order rather than anything meaningful to the user.
  get columns() {
    const columns = [
      {
        label: 'Instance',
        sortKey: 'name',
        sortValue: (item) => (item.Service?.ID || '').toLowerCase(),
      },
      { label: 'Service health' },
      { label: 'Service mesh' },
    ];

    if (!this.isNodeView) {
      columns.push({
        label: 'Node',
        sortKey: 'node',
        sortValue: (item) => (item.Node?.Node || '').toLowerCase(),
      });
      columns.push({ label: 'Node health' });
    }

    columns.push({ label: 'Address' });
    columns.push({ label: 'External source' });

    return columns;
  }

  // Mirrors the link param logic from Consul::Service::Table so that
  // cross-partition / cross-namespace / peered links keep working.
  //
  // The active partition/nspace args can be undefined (e.g. CE routes have no
  // partition segment), while item.Partition/Namespace default to 'default', so
  // both sides are normalized before comparing. Without this, the default case
  // would wrongly append partition/nspace params and produce a broken link.
  linkParams = (item) => {
    const hash = {};

    const argPartition = this.args.partition || 'default';
    const argNspace = this.args.nspace || 'default';
    const itemPartition = item.Partition || 'default';
    const itemNspace = item.Namespace || 'default';

    if (argPartition !== itemPartition) {
      hash.partition = itemPartition;
      hash.nspace = itemNspace;
    } else if (argNspace !== itemNspace) {
      hash.nspace = itemNspace;
    }

    if (item.Service?.PeerName) {
      hash.peer = item.Service.PeerName;
    }

    return hash;
  };
}
