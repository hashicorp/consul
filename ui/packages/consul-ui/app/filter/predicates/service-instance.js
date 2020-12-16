import setHelpers from 'mnemonist/set';

export default {
  statuses: {
    passing: (item, value) => item.Status === value,
    warning: (item, value) => item.Status === value,
    critical: (item, value) => item.Status === value,
  },
  sources: (item, values) => {
    return setHelpers.intersectionSize(values, new Set(item.ExternalSources || [])) !== 0;
  },
};
