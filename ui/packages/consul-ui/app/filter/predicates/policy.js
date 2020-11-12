import setHelpers from 'mnemonist/set';
export default () => ({ dcs = [], kinds = [] }) => {
  const kindIncludes = ['global-management', 'standard'].reduce((prev, item) => {
    prev[item] = kinds.includes(item);
    return prev;
  }, {});
  const selectedDcs = new Set(dcs);
  return item => {
    let kind = true;
    let dc = true;
    if (kinds.length > 0) {
      kind = false;
      if (kindIncludes['global-management'] && item.isGlobalManagement) {
        kind = true;
      }
      if (kindIncludes['standard'] && !item.isGlobalManagement) {
        kind = true;
      }
    }
    if (dcs.length > 0) {
      // if datacenters is undefined it means the policy is applicable to all datacenters
      dc =
        typeof item.Datacenters === 'undefined' ||
        setHelpers.intersectionSize(selectedDcs, new Set(item.Datacenters)) > 0;
    }
    return kind && dc;
  };
};
