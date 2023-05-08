import setHelpers from 'mnemonist/set';

export default {
  kind: {
    'global-management': (item, value) => item.isGlobalManagement,
    standard: (item, value) => !item.isGlobalManagement,
  },
  datacenter: (item, values) => {
    return (
      typeof item.Datacenters === 'undefined' ||
      setHelpers.intersectionSize(values, new Set(item.Datacenters)) > 0
    );
  },
};
