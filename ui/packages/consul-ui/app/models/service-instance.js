/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import { computed } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Node.Node,Service.ID';

export const Collection = class Collection {
  @tracked items;

  constructor(items) {
    this.items = items;
  }

  get ExternalSources() {
    const sources = this.items.reduce(function (prev, item) {
      return prev.concat(item.ExternalSources || []);
    }, []);
    // unique, non-empty values, alpha sort
    return [...new Set(sources)].filter(Boolean).sort();
  }
};

export default class ServiceInstance extends Model {
  @attr('string') uid;

  @attr('string') Datacenter;
  // Proxy is the actual JSON api response
  @attr() Proxy;
  @attr() Node;
  @attr() Service;
  @fragmentArray('health-check') Checks;
  @attr('number') SyncTime;
  @attr() meta;
  @attr({ defaultValue: () => [] }) Resources; // []

  // The name is the Name of the Service (the grouping of instances)
  get Name() {
    return this.Service?.Service;
  }

  // If the ID is blank fallback to the Service.Service (the Name)
  get ID() {
    return this.Service?.ID || this.Service?.Service;
  }
  get Address() {
    return this.Service?.Address || this.Node?.Address;
  }
  @attr('string') SocketPath;

  get Tags() {
    return this.Service?.Tags;
  }
  get Meta() {
    return this.Service?.Meta;
  }
  get Namespace() {
    return this.Service?.Namespace;
  }
  get Partition() {
    return this.Service?.Partition;
  }

  get ServiceChecks() {
    return this.Checks.filter((item) => item.Kind === 'service');
  }
  get NodeChecks() {
    return this.Checks.filter((item) => item.Kind === 'node');
  }

  @computed('Service.Meta')
  get ExternalSources() {
    const sources = Object.entries(this.Service.Meta || {})
      .filter(([key, value]) => key === 'external-source')
      .map(([key, value]) => {
        return value;
      });
    return [...new Set(sources)];
  }

  @computed('Service.Kind')
  get IsProxy() {
    return [
      'connect-proxy',
      'mesh-gateway',
      'ingress-gateway',
      'terminating-gateway',
      'api-gateway',
    ].includes(this.Service.Kind);
  }

  // IsOrigin means that the service can have associated up or down streams,
  // this service being the origin point of those streams
  @computed('Service.Kind')
  get IsOrigin() {
    return !['connect-proxy', 'mesh-gateway'].includes(this.Service.Kind);
  }

  // IsMeshOrigin means that the service can have associated up or downstreams
  // that are in the Consul mesh itself
  @computed('IsOrigin', 'Service.Kind')
  get IsMeshOrigin() {
    return this.IsOrigin && !['terminating-gateway'].includes(this.Service.Kind);
  }

  @computed('ChecksCritical.[]', 'ChecksPassing.[]', 'ChecksWarning.[]')
  get Status() {
    switch (true) {
      case this.ChecksCritical.length !== 0:
        return 'critical';
      case this.ChecksWarning.length !== 0:
        return 'warning';
      case this.ChecksPassing.length !== 0:
        return 'passing';
      default:
        return 'empty';
    }
  }

  @computed('Checks.[]')
  get ChecksPassing() {
    return this.Checks.filter((item) => item.Status === 'passing');
  }

  @computed('Checks.[]')
  get ChecksWarning() {
    return this.Checks.filter((item) => item.Status === 'warning');
  }

  @computed('Checks.[]')
  get ChecksCritical() {
    return this.Checks.filter((item) => item.Status === 'critical');
  }

  @computed('Checks.[]', 'ChecksPassing.[]')
  get PercentageChecksPassing() {
    return (this.ChecksPassing.length / this.Checks.length) * 100;
  }

  @computed('Checks.[]', 'ChecksWarning.[]')
  get PercentageChecksWarning() {
    return (this.ChecksWarning.length / this.Checks.length) * 100;
  }

  @computed('Checks.[]', 'ChecksCritical.[]')
  get PercentageChecksCritical() {
    return (this.ChecksCritical.length / this.Checks.length) * 100;
  }
}
