/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr, belongsTo, defaultValue } from '@ember-data/model';
import { computed } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { fragment } from 'ember-data-model-fragments/attributes';
import replace, { nullValue } from 'consul-ui/decorators/replace';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name,PeerName';

export const Collection = class Collection {
  @tracked items;

  constructor(items) {
    this.items = items;
  }

  get ExternalSources() {
    const items = this.items.reduce(function (prev, item) {
      return prev.concat(item.ExternalSources || []);
    }, []);
    // unique, non-empty values, alpha sort
    return [...new Set(items)].filter(Boolean).sort();
  }
  // TODO: Think about when this/collections is worthwhile using and explain
  // when and when not somewhere in the docs
  get Partitions() {
    // unique, non-empty values, alpha sort
    return [...new Set(this.items.map((item) => item.Partition))].sort();
  }
};
export default class Service extends Model {
  @attr('string') uid;
  @attr('string') Name;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string') Kind;
  @replace('', undefined) @attr('string') PeerName;
  @attr('number') ChecksPassing;
  @attr('number') ChecksCritical;
  @attr('number') ChecksWarning;
  @attr('number', { defaultValue: () => 0 }) InstancesPassing;
  @attr('number', { defaultValue: () => 0 }) InstancesCritical;
  @attr('number', { defaultValue: () => 0 }) InstancesWarning;
  @attr('number', { defaultValue: () => 0 }) MarkServiceStatusThreshold;
  @attr('number', { defaultValue: () => 0 }) InstanceCount;
  @attr('boolean') ConnectedWithGateway;
  @attr('boolean') ConnectedWithProxy;
  @attr({ defaultValue: () => [] }) Resources; // []
  @attr('number') SyncTime;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;

  @nullValue([]) @attr({ defaultValue: () => [] }) Tags;

  @attr() Nodes; // array
  @attr() Proxy; // Service
  @fragment('gateway-config') GatewayConfig;
  @nullValue([]) @attr() ExternalSources; // array
  @attr() Meta; // {}

  @attr() meta; // {}

  @belongsTo({ async: false }) peer;

  @computed('peer', 'InstanceCount')
  get isZeroCountButPeered() {
    return this.peer && this.InstanceCount === 0;
  }

  @computed('peer.State')
  get peerIsFailing() {
    return this.peer && this.peer.State === 'FAILING';
  }

  @computed('ChecksPassing', 'ChecksWarning', 'ChecksCritical')
  get ChecksTotal() {
    return this.ChecksPassing + this.ChecksWarning + this.ChecksCritical;
  }

  @computed('MeshChecksPassing', 'MeshChecksWarning', 'MeshChecksCritical')
  get MeshChecksTotal() {
    return this.MeshChecksPassing + this.MeshChecksWarning + this.MeshChecksCritical;
  }

  /* Mesh properties involve both the service and the associated proxy */
  @computed('ConnectedWithProxy', 'ConnectedWithGateway')
  get MeshEnabled() {
    return this.ConnectedWithProxy || this.ConnectedWithGateway;
  }

  @computed('MeshEnabled', 'Kind')
  get InMesh() {
    return this.MeshEnabled || (this.Kind || '').length > 0;
  }

  @computed(
    'MeshChecksPassing',
    'MeshChecksWarning',
    'MeshChecksCritical',
    'isZeroCountButPeered',
    'peerIsFailing'
  )
  get MeshStatus() {
    switch (true) {
      case this.isZeroCountButPeered:
        return 'unknown';
      case this.peerIsFailing:
        return 'unknown';
      case this.MeshInstancesPassing > 0 &&
        this.InstanceCount > 0 &&
        this.MeshInstancesPassing / this.InstanceCount >= this.MarkServiceStatusThreshold:
        return 'passing';
      case this.MeshInstancesCritical !== 0:
        return 'critical';
      case this.MeshInstancesWarning !== 0:
        return 'warning';
      default:
        return 'empty';
    }
  }

  @computed('isZeroCountButPeered', 'peerIsFailing', 'MeshStatus')
  get healthTooltipText() {
    const { MeshStatus, isZeroCountButPeered, peerIsFailing } = this;
    if (isZeroCountButPeered) {
      return 'This service currently has 0 instances. Check with the operator of its peer to make sure this is expected behavior.';
    }
    if (peerIsFailing) {
      return 'This peer is out of sync, so the current health statuses of its services are unknown.';
    }
    if (MeshStatus === 'critical') {
      if (this.MeshInstancesPassing === 0) {
        return `No instance has all health checks passing. At least one health check on one instance is failing.`;
      } else {
        return `Less than ${
          this.MarkServiceStatusThreshold * 100
        }% of total instances have all health checks passing. At least one health check on one instance is failing.`;
      }
    }
    if (MeshStatus === 'warning') {
      if (this.MeshInstancesPassing === 0) {
        return `No instance has all health checks passing. At least one health check on one instance has a warning.`;
      } else {
        return `Less than ${
          this.MarkServiceStatusThreshold * 100
        }% of total instances have all health checks passing. At least one health check on one instance has a warning.`;
      }
    }
    if (MeshStatus == 'passing') {
      if (this.MeshInstancesPassing === this.InstanceCount) {
        return 'All health checks on all instances are passing.';
      } else {
        return `Atleaset ${
          this.MarkServiceStatusThreshold * 100
        }% of instances have all health checks passing.`;
      }
    }
    return 'There are no health checks';
  }

  @computed('ChecksPassing', 'Proxy.ChecksPassing')
  get MeshChecksPassing() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.ChecksPassing;
    }
    return this.ChecksPassing + proxyCount;
  }

  @computed('ChecksWarning', 'Proxy.ChecksWarning')
  get MeshChecksWarning() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.ChecksWarning;
    }
    return this.ChecksWarning + proxyCount;
  }

  @computed('ChecksCritical', 'Proxy.ChecksCritical')
  get MeshChecksCritical() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.ChecksCritical;
    }
    return this.ChecksCritical + proxyCount;
  }

  @computed('InstancesPassing', 'Proxy.InstancesPassing')
  get MeshInstancesPassing() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.InstancesPassing;
    }
    return this.InstancesPassing + proxyCount;
  }

  @computed('InstancesWarning', 'Proxy.InstancesWarning')
  get MeshInstancesWarning() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.InstancesWarning;
    }
    return this.InstancesWarning + proxyCount;
  }

  @computed('InstancesCritical', 'Proxy.InstancesCritical')
  get MeshInstancesCritical() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.InstancesCritical;
    }
    return this.InstancesCritical + proxyCount;
  }

  /**/
}
