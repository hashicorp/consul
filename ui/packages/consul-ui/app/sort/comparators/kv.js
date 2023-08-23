export default ({ properties }) => key => {
  return properties(['Key', 'Kind'])(key);
};
