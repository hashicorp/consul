export default (search, secondary = () => {}) => scope => {
  return {
    scope: scope,
    ...search(),
    ...secondary(),
  };
};
