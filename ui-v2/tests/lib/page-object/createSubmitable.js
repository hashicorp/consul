export default function(clickable, is) {
  return function(obj) {
    return {
      ...obj,
      ...{
        submit: clickable('[type=submit]'),
        submitIsEnabled: is(':not(:disabled)', '[type=submit]'),
      },
    };
  };
}
