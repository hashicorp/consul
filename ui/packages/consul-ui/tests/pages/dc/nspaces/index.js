export default function (visitable, creatable, nspaces, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/namespaces'),
    nspaces: nspaces(),
    sort: popoverSelect('[data-test-sort-control]'),
  });
}
