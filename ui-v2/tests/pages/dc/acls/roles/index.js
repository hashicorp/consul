export default function(visitable, creatable, roles, filter) {
  return {
    visit: visitable('/:dc/acls/roles'),
    roles: roles(),
    filter: filter(),
    ...creatable(),
  };
}
