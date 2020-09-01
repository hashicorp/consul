import setHelpers from 'mnemonist/set';
export default () => ({ dcs = [], types = [] }) => {
  const typeIncludes = ['global-management', 'standard'].reduce((prev, item) => {
    prev[item] = types.includes(item);
    return prev;
  }, {});
  const selectedDcs = new Set(dcs);
  return item => {
    let type = true;
    let dc = true;
    if (types.length > 0) {
      type = false;
      if (typeIncludes['global-management'] && item.isGlobalManagement) {
        type = true;
      }
      if (typeIncludes['standard'] && !item.isGlobalManagement) {
        type = true;
      }
    }
    if (dcs.length > 0) {
      // if datacenters is undefined it means the policy is applicable to all datacenters
      dc =
        typeof item.Datacenters === 'undefined' ||
        setHelpers.intersectionSize(selectedDcs, new Set(item.Datacenters)) > 0;
    }
    return type && dc;
  };
};
