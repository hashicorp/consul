export default function(visitable, attribute, collection, text, filter) {
  return {
    visit: visitable('/:dc/services/:service'),
    nodes: collection('[data-test-node]', {
      name: attribute('data-test-node'),
    }),
    healthy: collection('[data-test-healthy] [data-test-node]', {
      name: attribute('data-test-node'),
      address: text('header strong'),
    }),
    unhealthy: collection('[data-test-unhealthy] [data-test-node]', {
      name: attribute('data-test-node'),
      address: text('header strong'),
    }),
    filter: filter,
  };
}
