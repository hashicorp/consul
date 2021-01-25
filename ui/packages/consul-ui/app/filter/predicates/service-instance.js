import setHelpers from 'mnemonist/set';

export default {
  status: {
    passing: (item, value) => item.Status === value,
    warning: (item, value) => item.Status === value,
    critical: (item, value) => item.Status === value,
  },
  source: (item, values) => {
    return setHelpers.intersectionSize(values, new Set(item.ExternalSources || [])) !== 0;
  },
};
