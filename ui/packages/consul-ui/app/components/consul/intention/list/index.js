import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { sort } from '@ember/object/computed';

export default class ConsulIntentionList extends Component {

  @service('filter') filter;
  @service('sort') sort;
  @service('search') search;
  @service('repository/intention') repo;

  @sort('searched', 'comparator') sorted;

  @tracked isManagedByCRDs;

  constructor(owner, args) {
    super(...arguments);
    this.updateCRDManagement(args.items);
  }
  get items() {
    return this.sorted;
  }
  get filtered() {
    const predicate = this.filter.predicate('intention');
    return this.args.items.filter(predicate(this.args.filters))
  }
  get searched() {
    if(typeof this.args.search === 'undefined') {
      return this.filtered;
    }
    const predicate = this.search.predicate('intention');
    return this.filtered.filter(predicate(this.args.search));
  }
  get comparator() {
    return [this.args.sort];
  }
  get checkedItem() {
    if(this.searched.length === 1) {
      return this.searched[0].SourceName === this.args.search ? this.searched[0] : null;
    }
    return null;
  }
  @action
  updateCRDManagement() {
    this.isManagedByCRDs = this.repo.isManagedByCRDs();
  }
}
