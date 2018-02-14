import Controller from '@ember/controller';
import { computed } from '@ember/object';
export function initialize(/* application */) {
  Controller.reopen({
    needs: ['dc', 'application'],
    // queryParams: ["filter", "status", "condensed"],
    // dc: computed.alias("controllers.dc"),
    condensed: true,
    hasExpanded: true,
    filterText: 'Filter by name',
    filter: '', // default
    status: 'any status', // default
    statuses: ['any status', 'passing', 'failing'],
    isShowingItem: function() {
      var currentPath = this.get('controllers.application.currentPath');
      return currentPath === 'dc.nodes.show' || currentPath === 'dc.services.show';
    }.property('controllers.application.currentPath'),
    filteredContent: function() {
      var filter = this.get('filter');
      var status = this.get('status');

      var items = this.get('items').filter(function(item) {
        return item
          .get('filterKey')
          .toLowerCase()
          .match(filter.toLowerCase());
      });
      switch (status) {
        case 'passing':
          return items.filterBy('hasFailingChecks', false);
        case 'failing':
          return items.filterBy('hasFailingChecks', true);
        default:
          return items;
      }
    }.property('filter', 'status', 'items.@each'),
    actions: {
      toggleCondensed: function() {
        this.toggleProperty('condensed');
      },
    },
  });
}

export default {
  initialize,
};
