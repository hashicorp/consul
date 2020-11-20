export default triggerable => () => {
  return {
    ...{
      search: triggerable('keypress', '[name="s"]'),
    },
  };
};
