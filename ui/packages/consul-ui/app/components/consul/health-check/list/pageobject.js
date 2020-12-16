export default (collection, text) => (scope = '.consul-health-check-list') => {
  return {
    scope,
    item: collection('li', {
      name: text('header h3'),
      type: text('[data-health-check-type]'),
      exposed: text('[data-test-exposed]'),
    }),
  };
};
