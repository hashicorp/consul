export default function(clickable, is) {
  return function(obj) {
    return {
      ...obj,
      ...{
        cancel: clickable('[type=reset]'),
        cancelIsEnabled: is(':not(:disabled)', '[type=reset]'),
      },
    };
  };
}
