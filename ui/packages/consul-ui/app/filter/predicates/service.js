import setHelpers from 'mnemonist/set';
export default () => ({ instances = [], sources = [], statuses = [], kinds = [] }) => {
  const uniqueSources = new Set(sources);
  const kindIncludes = [
    'ingress-gateway',
    'terminating-gateway',
    'mesh-gateway',
    'service',
    'in-mesh',
    'not-in-mesh',
  ].reduce((prev, item) => {
    prev[item] = kinds.includes(item);
    return prev;
  }, {});
  const instanceIncludes = ['registered', 'not-registered'].reduce((prev, item) => {
    prev[item] = instances.includes(item);
    return prev;
  }, {});
  return item => {
    if (statuses.length > 0) {
      if (statuses.includes(item.MeshStatus)) {
        return true;
      }
      return false;
    }
    if (instances.length > 0) {
      if (item.InstanceCount > 0) {
        if (instanceIncludes['registered']) {
          return true;
        }
      } else {
        if (instanceIncludes['not-registered']) {
          return true;
        }
      }
      return false;
    }
    if (kinds.length > 0) {
      if (kindIncludes['ingress-gateway'] && item.Kind === 'ingress-gateway') {
        return true;
      }
      if (kindIncludes['terminating-gateway'] && item.Kind === 'terminating-gateway') {
        return true;
      }
      if (kindIncludes['mesh-gateway'] && item.Kind === 'mesh-gateway') {
        return true;
      }
      if (kindIncludes['service'] && typeof item.Kind === 'undefined') {
        return true;
      }
      if (kindIncludes['in-mesh']) {
        if (item.InMesh) {
          return true;
        }
      }
      if (kindIncludes['not-in-mesh']) {
        if (!item.InMesh) {
          return true;
        }
      }
      return false;
    }
    if (sources.length > 0) {
      if (setHelpers.intersectionSize(uniqueSources, new Set(item.ExternalSources || [])) !== 0) {
        return true;
      }
      return false;
    }
    return true;
  };
};
