export default function(model, props, attr = {}) {
  model.eachAttribute(function(item) {
    attr[item] = {
      ...attr[item],
      ...{
        serialize: props.indexOf(item) !== -1,
      },
    };
  });
  return attr;
}
