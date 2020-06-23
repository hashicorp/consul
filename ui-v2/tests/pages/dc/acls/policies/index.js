export default function(visitable, creatable, policies, filter) {
  return creatable({
    visit: visitable('/:dc/acls/policies'),
    policies: policies(),
    filter: filter(),
  });
}
