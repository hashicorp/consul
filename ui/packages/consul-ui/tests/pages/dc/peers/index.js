import tabgroup from 'consul-ui/components/tab-nav/pageobject';

export default function (visitable, creatable, items, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/peers'),
    peers: items(),
    sort: popoverSelect('[data-test-sort-control]'),
    tabs: tabgroup('tab', ['imported-services', 'exported-services', 'server-addresses']),
  });
}
