export default function(visitable, creatable, intentions, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/intentions'),
    intentions: intentions(),
    sort: popoverSelect(),
  });
}
