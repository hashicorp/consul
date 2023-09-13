export default (collection, clickable, attribute, text, actions) => () => {
  return collection('.consul-nspace-list [data-test-list-row]', {
    nspace: clickable('a'),
    name: attribute('data-test-nspace', '[data-test-nspace]'),
    description: text('[data-test-description]'),
    ...actions(['edit', 'delete']),
  });
};
