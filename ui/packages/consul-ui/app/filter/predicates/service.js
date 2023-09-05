/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import setHelpers from 'mnemonist/set';

export default {
  kind: {
    'api-gateway': (item, value) => item.Kind === value,
    'ingress-gateway': (item, value) => item.Kind === value,
    'terminating-gateway': (item, value) => item.Kind === value,
    'mesh-gateway': (item, value) => item.Kind === value,
    service: (item, value) => !item.Kind,
    'in-mesh': (item, value) => item.InMesh,
    'not-in-mesh': (item, value) => !item.InMesh,
  },
  status: {
    passing: (item, value) => item.MeshStatus === value,
    warning: (item, value) => item.MeshStatus === value,
    critical: (item, value) => item.MeshStatus === value,
    empty: (item, value) => item.MeshChecksTotal === 0,
    unknown: (item) => item.peerIsFailing || item.isZeroCountButPeered,
  },
  instance: {
    registered: (item, value) => item.InstanceCount > 0,
    'not-registered': (item, value) => item.InstanceCount === 0,
  },
  source: (item, values) => {
    let includeBecauseNoExternalSourcesOrPeered = false;

    if (values.includes('consul')) {
      includeBecauseNoExternalSourcesOrPeered =
        !item.ExternalSources ||
        item.ExternalSources.length === 0 ||
        (item.ExternalSources.length === 1 && item.ExternalSources[0] === '') ||
        item.PeerName;
    }

    return (
      setHelpers.intersectionSize(values, new Set(item.ExternalSources || [])) !== 0 ||
      values.includes(item.Partition) ||
      includeBecauseNoExternalSourcesOrPeered
    );
  },
};
