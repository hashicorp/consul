export default ({ properties }) => key => {
  return properties(['CreateTime'])(key);
};
