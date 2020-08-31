export default function(visitable, creatable, roles, popoverSelect) {
  return {
    visit: visitable('/:dc/acls/roles'),
    roles: roles(),
    sort: popoverSelect(),
    ...creatable(),
  };
}
