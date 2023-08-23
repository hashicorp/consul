import Component from '@ember/component';
import { get, set, computed } from '@ember/object';
import { alias, not, equal } from '@ember/object/computed';
import { inject as service } from '@ember/service';

const name = 'intention-permission';
export default Component.extend({
  tagName: '',
  name: name,

  schema: service('schema'),
  change: service('change'),
  repo: service(`repository/${name}`),

  onsubmit: function() {},
  onreset: function() {},

  intents: alias(`schema.${name}.Action.allowedValues`),
  methods: alias(`schema.${name}-http.Methods.allowedValues`),
  pathProps: alias(`schema.${name}-http.PathType.allowedValues`),

  pathTypes: computed('pathProps', function() {
    return ['NoPath'].concat(this.pathProps);
  }),

  pathLabels: computed(function() {
    return {
      NoPath: 'No Path',
      PathExact: 'Exact',
      PathPrefix: 'Prefixed by',
      PathRegex: 'Regular Expression',
    };
  }),

  pathInputLabels: computed(function() {
    return {
      PathExact: 'Exact Path',
      PathPrefix: 'Path Prefix',
      PathRegex: 'Path Regular Expression',
    };
  }),

  changeset: computed('item', function() {
    const changeset = this.change.changesetFor(name, this.item || this.repo.create());
    if (changeset.isNew) {
      changeset.validate();
    }
    return changeset;
  }),

  pathType: computed('changeset._changes.HTTP.PathType', 'pathTypes.firstObject', function() {
    return this.changeset.HTTP.PathType || this.pathTypes.firstObject;
  }),
  noPathType: equal('pathType', 'NoPath'),
  shouldShowPathField: not('noPathType'),

  allMethods: false,
  shouldShowMethods: not('allMethods'),

  didReceiveAttrs: function() {
    if (!get(this, 'item.HTTP.Methods.length')) {
      set(this, 'allMethods', true);
    }
  },

  actions: {
    change: function(name, changeset, e) {
      const value = typeof get(e, 'target.value') !== 'undefined' ? e.target.value : e;
      switch (name) {
        case 'allMethods':
          set(this, name, e.target.checked);
          break;
        case 'method':
          if (e.target.checked) {
            this.actions.add.apply(this, ['HTTP.Methods', changeset, value]);
          } else {
            this.actions.delete.apply(this, ['HTTP.Methods', changeset, value]);
          }
          break;
        default:
          changeset.set(name, value);
      }
      changeset.validate();
    },
    add: function(prop, changeset, value) {
      changeset.pushObject(prop, value);
      changeset.validate();
    },
    delete: function(prop, changeset, value) {
      changeset.removeObject(prop, value);
      changeset.validate();
    },
    submit: function(changeset, e) {
      const pathChanged =
        typeof changeset.changes.find(
          ({ key, value }) => key === 'HTTP.PathType' || key === 'HTTP.Path'
        ) !== 'undefined';
      if (pathChanged) {
        this.pathProps.forEach(prop => {
          changeset.set(`HTTP.${prop}`, undefined);
        });
        if (changeset.HTTP.PathType !== 'NoPath') {
          changeset.set(`HTTP.${changeset.HTTP.PathType}`, changeset.HTTP.Path);
        }
      }

      if (this.allMethods) {
        changeset.set('HTTP.Methods', null);
      }
      // this will prevent the changeset from overwriting the
      // computed properties on the ED object
      delete changeset._changes.HTTP.PathType;
      delete changeset._changes.HTTP.Path;
      //
      this.repo.persist(changeset);
      this.onsubmit(changeset.data);
    },
    reset: function(changeset, e) {
      changeset.rollback();
      this.onreset(changeset.data);
    },
  },
});
