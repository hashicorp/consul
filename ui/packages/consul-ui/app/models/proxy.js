/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { attr } from '@ember-data/model';
import ServiceInstanceModel from './service-instance';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Node,ServiceID';

// TODO: This should be changed to ProxyInstance
export default class ProxyServiceInstance extends ServiceInstanceModel {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;

  @attr('string') ServiceName;
  @attr('string') ServiceID;
  @attr('string') NodeName;
  @attr('number') SyncTime;
  @attr() ServiceProxy; // {}
}
