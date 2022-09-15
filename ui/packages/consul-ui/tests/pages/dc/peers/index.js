export default function (visitable, creatable, items, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/peers'),
    peers: items(),
    sort: popoverSelect('[data-test-sort-control]'),
  });
}
