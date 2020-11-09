import setHelpers from 'mnemonist/set';
export default () => ({ instances = [], sources = [], statuses = [], types = [] }) => {
  const uniqueSources = new Set(sources);
  const typeIncludes = [
    'ingress-gateway',
    'terminating-gateway',
    'mesh-gateway',
    'service',
    'in-mesh',
    'not-in-mesh',
  ].reduce((prev, item) => {
    prev[item] = types.includes(item);
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
    if (types.length > 0) {
      if (typeIncludes['ingress-gateway'] && item.Kind === 'ingress-gateway') {
        return true;
      }
      if (typeIncludes['terminating-gateway'] && item.Kind === 'terminating-gateway') {
        return true;
      }
      if (typeIncludes['mesh-gateway'] && item.Kind === 'mesh-gateway') {
        return true;
      }
      if (typeIncludes['service'] && typeof item.Kind === 'undefined') {
        return true;
      }
      if (typeIncludes['in-mesh']) {
        if (item.InMesh) {
          return true;
        }
      }
      if (typeIncludes['not-in-mesh']) {
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
