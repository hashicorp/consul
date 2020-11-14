export default function(clickable, is) {
  return function(obj) {
    return {
      ...obj,
      ...{
        create: clickable('[data-test-create]'),
        createIsEnabled: is(':not(:disabled)', '[data-test-create]'),
      },
    };
  };
}
