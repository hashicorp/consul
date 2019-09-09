export default function(visitable, clickable, attribute, collection, filter) {
  const node = {
    name: attribute('data-test-node'),
    leader: attribute('data-test-leader', '[data-test-leader]'),
    node: clickable('header a'),
  };
  return {
    visit: visitable('/:dc/nodes'),
    nodes: collection('[data-test-node]', node),
    healthyNodes: collection('.healthy [data-test-node]', node),
    unHealthyNodes: collection('.unhealthy [data-test-node]', node),
    filter: filter,
  };
}
