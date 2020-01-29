export default function(visitable, submitable, isVisible) {
  return submitable({
    visit: visitable('/setting'),
    blockingQueries: isVisible('[data-test-blocking-queries]'),
  });
}
