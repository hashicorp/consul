/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';
import mergeChecks from '../../utils/merge-checks';
import { serviceExternalSource } from './external-source';
import { CUT_SERVICE_LIST_ITEM_TYPE } from '@hashicorp/consul-ui-toolkit/utils/service-list-item';

// will be used from cut import
export const ServiceListItemType = {
  ServiceInstance: 'service-instance',
  Service: 'service',
};

function serviceInstanceListItem(serviceInstance, node, proxy, isExternalSource) {
  const checks = mergeChecks([serviceInstance.Checks, proxy?.Checks || []]);
  const nodeChecks = checks?.filter((item) => item.ServiceID === '');
  const serviceChecks = checks?.filter((item) => item.ServiceID !== '');
  const serviceAddress = serviceInstance.Service?.Address;
  const nodeAddress = serviceInstance.Node?.Address;
  const servicePort = serviceInstance.Service?.Port;
  const address = serviceAddress || nodeAddress;

  return {
    name: serviceInstance.Service.ID,
    metadata: {
      healthCheck: {
        node: {
          success: nodeChecks.filterBy('Status', 'passing').length,
          warning: nodeChecks.filterBy('Status', 'warning').length,
          critical: nodeChecks.filterBy('Status', 'critical').length,
        },
        service: {
          success: serviceChecks.filterBy('Status', 'passing').length,
          warning: serviceChecks.filterBy('Status', 'warning').length,
          critical: serviceChecks.filterBy('Status', 'critical').length,
        },
      },
      tags: serviceInstance.Service.Tags,
      servicePortAddress: servicePort ? `${address}:${servicePort}` : null,
      serviceSocketPath: serviceInstance.Service.SocketPath,
      node:
        !node && !serviceInstance.Node?.Meta?.['synthetic-node']
          ? serviceInstance.Node?.Node
          : null,
      externalSource:
        node || isExternalSource ? serviceExternalSource([serviceInstance.Service]) : null,
      connectedWithProxy: !!proxy,
    },
  };
}

function serviceListItem(service) {
  return {
    name: service.Name,
    metadata: {
      instanceCount: service.InstanceCount,
      upstreamCount: service.GatewayConfig?.AssociatedServiceCount,
      linkedServiceCount: service.GatewayConfig?.AssociatedServiceCount,
      kind: service.Kind,
      healthCheck: {
        instance: {
          success: service.MeshChecksPassing,
          warning: service.MeshChecksWarning,
          critical: service.MeshChecksCritical,
        },
      },
      connectedWithGateway: service.ConnectedWithGateway,
      connectedWithProxy: service.ConnectedWithProxy,
      tags: service.Tags,
      externalSource: serviceExternalSource([service]),
    },
  };
}

export default helper(function transformDataToListItem(
  [service, type, node, proxy, isExternalSource] /*, hash*/
) {
  switch (type) {
    case CUT_SERVICE_LIST_ITEM_TYPE.ServiceInstance:
      return serviceInstanceListItem(service, node, proxy, isExternalSource);
    case CUT_SERVICE_LIST_ITEM_TYPE.Service: {
      return serviceListItem(service);
    }
    default:
      return null;
  }
});
