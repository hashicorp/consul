export default function() {
  const props = [].slice.call(arguments);
  return function() {
    const vals = [].slice.call(arguments);
    const pojo = {};
    props.forEach((item, i /*, arr */) => {
      pojo[item] = vals[i];
    });
    return pojo;
  };
}
