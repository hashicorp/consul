/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';
import mergeChecks from '../../utils/merge-checks';
import { serviceExternalSource } from './external-source';
import { titleize } from 'ember-cli-string-helpers/helpers/titleize';
import { humanize } from 'ember-cli-string-helpers/helpers/humanize';

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

const normalizedGatewayLabels = {
  'api-gateway': 'API Gateway',
  'mesh-gateway': 'Mesh Gateway',
  'ingress-gateway': 'Ingress Gateway',
  'terminating-gateway': 'Terminating Gateway',
};

function serviceListItem(service) {
  const kind = service.Kind;
  let kindName = normalizedGatewayLabels[kind];
  kindName = kindName || (kind ? titleize(humanize(kind)) : undefined);

  return {
    name: service.Name,
    metadata: {
      kind: service.Kind,
      kindName,
      instanceCount: ['terminating-gateway', 'ingress-gateway'].includes(service.Kind)
        ? undefined
        : service.InstanceCount,
      linkedServiceCount:
        service.Kind === 'terminating-gateway'
          ? service.GatewayConfig.AssociatedServiceCount
          : undefined,
      upstreamCount:
        service.Kind === 'ingress-gateway'
          ? service.GatewayConfig.AssociatedServiceCount
          : undefined,
      externalSource: serviceExternalSource([service]),
      healthCheck: {
        isInstanceChecks: true,
        instance: {
          success: service.MeshChecksPassing,
          warning: service.MeshChecksWarning,
          critical: service.MeshChecksCritical,
        },
      },
      connectedWithGateway: service.ConnectedWithGateway,
      connectedWithProxy: service.ConnectedWithProxy,
    },
  };
}

export default helper(function transformDataToListItem(
  [service, type, node, proxy, isExternalSource] /*, hash*/
) {
  switch (type) {
    case ServiceListItemType.ServiceInstance:
      return serviceInstanceListItem(service, node, proxy, isExternalSource);
    case ServiceListItemType.Service: {
      return serviceListItem(service);
    }
    default:
      return null;
  }
});
