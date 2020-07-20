export default function(visitable, creatable, nspaces, filter) {
  return creatable({
    visit: visitable('/:dc/namespaces'),
    nspaces: nspaces(),
    filter: filter(),
  });
}
