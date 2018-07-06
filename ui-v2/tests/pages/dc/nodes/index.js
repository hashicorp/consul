export default function(visitable, clickable, attribute, collection, filter) {
  return {
    visit: visitable('/:dc/nodes'),
    nodes: collection('[data-test-node]', {
      name: attribute('data-test-node'),
      node: clickable('header a'),
    }),
    filter: filter,
  };
}
