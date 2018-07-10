export default function(clickable) {
  return function(obj) {
    return {
      ...obj,
      ...{
        delete: clickable('[data-test-delete]'),
        confirmDelete: clickable('button.type-delete'),
      },
    };
  };
}
