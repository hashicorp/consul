import setHelpers from 'mnemonist/set';
export default () => ({ sources = [], statuses = [] }) => {
  const uniqueSources = new Set(sources);
  return item => {
    if (statuses.length > 0) {
      if (statuses.includes(item.Status)) {
        return true;
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
