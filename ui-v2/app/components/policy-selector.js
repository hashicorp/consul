import ChildSelectorComponent from './child-selector';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
import updateArrayObject from 'consul-ui/utils/update-array-object';

const ERROR_PARSE_RULES = 'Failed to parse ACL rules';
const ERROR_INVALID_POLICY = 'Invalid service policy';
const ERROR_NAME_EXISTS = 'Invalid Policy: A Policy with Name';

export default ChildSelectorComponent.extend({
  repo: service('repository/policy/component'),
  datacenterRepo: service('repository/dc/component'),
  name: 'policy',
  type: 'policy',
  classNames: ['policy-selector'],
  init: function() {
    this._super(...arguments);
    const source = get(this, 'source');
    if (source) {
      const event = 'save';
      this.listen(source, event, e => {
        this.actions[event].bind(this)(...e.data);
      });
    }
  },
  reset: function(e) {
    this._super(...arguments);
    set(this, 'isScoped', false);
    set(this, 'datacenters', get(this, 'datacenterRepo').findAll());
  },
  refreshCodeEditor: function(e, target) {
    const selector = '.code-editor';
    get(this, 'dom')
      .component(selector, target)
      .didAppear();
  },
  error: function(e) {
    const item = get(this, 'item');
    const err = e.error;
    if (typeof err.errors !== 'undefined') {
      const error = err.errors[0];
      let prop = 'Rules';
      let message = error.detail;
      switch (true) {
        case message.indexOf(ERROR_PARSE_RULES) === 0:
        case message.indexOf(ERROR_INVALID_POLICY) === 0:
          prop = 'Rules';
          message = error.detail;
          break;
        case message.indexOf(ERROR_NAME_EXISTS) === 0:
          prop = 'Name';
          message = message.substr(ERROR_NAME_EXISTS.indexOf(':') + 1);
          break;
      }
      if (prop) {
        item.addError(prop, message);
      }
    } else {
      // TODO: Conponents can't throw, use onerror
      throw err;
    }
  },
  actions: {
    open: function(e) {
      this.refreshCodeEditor(e, e.target.parentElement);
    },
    loadItem: function(e, item, items) {
      const target = e.target;
      // the Details expander toggle, only load on opening
      if (target.checked) {
        const value = item;
        this.refreshCodeEditor(e, target.parentNode);
        if (get(item, 'template') === 'service-identity') {
          return;
        }
        // potentially the item could change between load, so we don't check
        // anything to see if its already loaded here
        const repo = get(this, 'repo');
        // TODO: Temporarily add dc here, will soon be serialized onto the policy itself
        const dc = get(this, 'dc');
        const slugKey = repo.getSlugKey();
        const slug = get(value, slugKey);
        updateArrayObject(items, repo.findBySlug(slug, dc), slugKey, slug);
      }
    },
  },
});
