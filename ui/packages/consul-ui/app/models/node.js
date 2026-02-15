/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr, hasMany } from '@ember-data/model';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default class Node extends Model {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Datacenter;
  @attr('string') PeerName;
  @attr('string') Partition;
  @attr('string') Address;
  @attr('string') Node;
  @attr('number') SyncTime;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr() meta; // {}
  @attr() Meta; // {}
  @attr() TaggedAddresses; // {lan, wan}
  @attr({ defaultValue: () => [] }) Resources; // []
  // Services are reshaped to a different shape to what you sometimes get from
  // the response, see models/node.js
  @hasMany('service-instance', { async: false, inverse: null }) Services; // TODO: Rename to ServiceInstances
  @fragmentArray('health-check') Checks;
  
  // MeshServiceInstances are all instances that aren't connect-proxies this
  // currently includes gateways as these need to show up in listings
  get MeshServiceInstances() {
    const services = this.Services;
    // Check if the relationship content is loaded before filtering
    if (!services || !services.length) {
      return [];
    }
    return services.filter((item) => item.Service.Kind !== 'connect-proxy');
  }
  
  // ProxyServiceInstances are all instances that are connect-proxies
  get ProxyServiceInstances() {
    const services = this.Services;
    // Check if the relationship content is loaded before filtering
    if (!services || !services.length) {
      return [];
    }
    return services.filter((item) => item.Service.Kind === 'connect-proxy');
  }

  get NodeChecks() {
    const checks = this.Checks;
    if (!checks || !checks.length) {
      return [];
    }
    return checks.filter((item) => item.ServiceID === '');
  }

  get Status() {
    switch (true) {
      case this.ChecksCritical !== 0:
        return 'critical';
      case this.ChecksWarning !== 0:
        return 'warning';
      case this.ChecksPassing !== 0:
        return 'passing';
      default:
        return 'empty';
    }
  }

  get ChecksCritical() {
    return this.NodeChecks.filter((item) => item.Status === 'critical').length;
  }

  get ChecksPassing() {
    return this.NodeChecks.filter((item) => item.Status === 'passing').length;
  }

  get ChecksWarning() {
    return this.NodeChecks.filter((item) => item.Status === 'warning').length;
  }

  get Version() {
    return this.Meta?.['consul-version'] ?? '';
  }
}
