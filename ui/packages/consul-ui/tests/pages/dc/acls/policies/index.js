export default function(visitable, creatable, policies, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/acls/policies'),
    policies: policies(),
    sort: popoverSelect('[data-test-sort-control]'),
  });
}
