export default ({ properties }) => key => {
  return properties(['Name'])(key);
};
