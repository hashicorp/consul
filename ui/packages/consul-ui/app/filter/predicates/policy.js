import setHelpers from 'mnemonist/set';

export default {
  kinds: {
    'global-management': (item, value) => item.isGlobalManagement,
    standard: (item, value) => !item.isGlobalManagement,
  },
  dcs: (item, values) => {
    return (
      typeof item.Datacenters === 'undefined' ||
      setHelpers.intersectionSize(values, new Set(item.Datacenters)) > 0
    );
  },
};
