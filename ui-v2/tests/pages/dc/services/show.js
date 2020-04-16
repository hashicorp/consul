export default function(
  visitable,
  clickable,
  attribute,
  collection,
  text,
  intentions,
  filter,
  tabs
) {
  return {
    visit: visitable('/:dc/services/:service'),
    externalSource: attribute('data-test-external-source', 'h1 span'),
    dashboardAnchor: {
      href: attribute('href', '[data-test-dashboard-anchor]'),
    },
    tabs: tabs('tab', ['instances', 'intentions', 'routing', 'tags']),
    filter: filter,

    // TODO: These need to somehow move to subpages
    instances: collection('#instances [data-test-tabular-row]', {
      address: text('[data-test-address]'),
    }),
    intentions: intentions(),
  };
}
