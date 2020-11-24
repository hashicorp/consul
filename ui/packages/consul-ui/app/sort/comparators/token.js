export default ({ properties }) => key => {
  return properties(['CreateIndex'])(key);
};
