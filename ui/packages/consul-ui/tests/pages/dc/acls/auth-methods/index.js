export default function(visitable, creatable, authMethods, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/acls/auth-methods'),
    authMethods: authMethods(),
    sort: popoverSelect('[data-test-sort-control]'),
  });
}
