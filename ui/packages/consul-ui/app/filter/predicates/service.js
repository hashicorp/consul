import setHelpers from 'mnemonist/set';

export default {
  kinds: {
    'ingress-gateway': (item, value) => item.Kind === value,
    'terminating-gateway': (item, value) => item.Kind === value,
    'mesh-gateway': (item, value) => item.Kind === value,
    service: (item, value) => !item.Kind,
    'in-mesh': (item, value) => item.InMesh,
    'not-in-mesh': (item, value) => !item.InMesh,
  },
  statuses: {
    passing: (item, value) => item.MeshStatus === value,
    warning: (item, value) => item.MeshStatus === value,
    critical: (item, value) => item.MeshStatus === value,
  },
  instances: {
    registered: (item, value) => item.InstanceCount > 0,
    'not-registered': (item, value) => item.InstanceCount === 0,
  },
  sources: (item, values) => {
    return setHelpers.intersectionSize(values, new Set(item.ExternalSources || [])) !== 0;
  },
};
