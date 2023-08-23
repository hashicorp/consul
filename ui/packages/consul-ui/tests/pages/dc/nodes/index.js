export default function (visitable, text, clickable, attribute, collection, popoverSelect) {
  const node = {
    name: text('[data-test-node]'),
    leader: attribute('data-test-leader', '[data-test-leader]'),
    node: clickable('a'),
    status: attribute('data-test-status', '[data-test-status]'),
  };
  return {
    visit: visitable('/:dc/nodes'),
    nodes: collection('.consul-node-list [data-test-list-row]', node),
    home: clickable('[data-test-home]'),
    sort: popoverSelect('[data-test-sort-control]'),
  };
}
