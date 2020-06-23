import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { setProperties, set, get, computed } from '@ember/object';
import { assert } from '@ember/debug';
import chart from './chart.xstate';

export default Component.extend({
  tagName: '',
  dom: service('dom'),
  builder: service('form'),
  init: function() {
    this._super(...arguments);
    this.chart = chart;
    this.form = this.builder.form('intention');
  },
  _item: computed('item', function() {
    return this.form.setData(this.item).getData();
  }),
  actions: {
    createServices: function(e) {
      // Services in the menus should:
      // 1. Be unique (they potentially
      // 2. Only include services that shold have intentions
      // 3. Include an 'All Services' option
      // 4. Include the current Source and Destination incase they are virtual services
      let items = e.data
        .uniqBy('Name')
        .toArray()
        .filter(
          item => !['connect-proxy', 'mesh-gateway', 'terminating-gateway'].includes(item.Kind)
        );
      let source = items.findBy('Name', this.item.SourceName);
      if (!source) {
        source = { Name: this.item.SourceName };
        items = [source].concat(items);
      }
      let destination = items.findBy('Name', this.item.DestinationName);
      if (!destination) {
        destination = { Name: this.item.DestinationName };
        items = [destination].concat(items);
      }
      items = [{ Name: '*' }].concat(items);
      setProperties(this, {
        services: items,
        SourceName: source,
        DestinationName: destination,
      });
    },
    createNspaces: function(e) {
      // Services in the menus should:
      // 1. Be unique
      // 2. Only include services that shold have intentions
      // 3. Include an 'All Services' option
      // 4. Include the current Source and Destination incase they are virtual services
      let items = e.data.toArray();
      let source = items.findBy('Name', this.item.SourceNS);
      if (!source) {
        source = { Name: this.item.SourceNS };
        items = [source].concat(items);
      }
      let destination = items.findBy('Name', this.item.DestinationNS);
      if (!destination) {
        destination = { Name: this.item.DestinationNS };
        items = [destination].concat(items);
      }
      items = [{ Name: '*' }].concat(items);
      setProperties(this, {
        nspaces: items,
        SourceNS: source,
        DestinationNS: destination,
      });
    },
    createNewLabel: function(template, term) {
      return template.replace(/{{term}}/g, term);
    },
    isUnique: function(items, term) {
      return !items.findBy('Name', term);
    },
    submit: function(item, e) {
      e.preventDefault();
      this.onsubmit(...arguments);
    },
    change: function(e, value, item) {
      const event = this.dom.normalizeEvent(e, value);
      const form = this.form;
      const target = event.target;

      let name, selected, match;
      switch (target.name) {
        case 'SourceName':
        case 'DestinationName':
        case 'SourceNS':
        case 'DestinationNS':
          name = selected = target.value;
          // Names can be selected Service EmberObjects or typed in strings
          // if its not a string, use the `Name` from the Service EmberObject
          if (typeof name !== 'string') {
            name = get(target.value, 'Name');
          }
          // mutate the value with the string name
          // which will be handled by the form
          target.value = name;
          // these are 'non-form' variables so not on `item`
          // these variables also exist in the template so we know
          // the current selection
          // basically the difference between
          // `item.DestinationName` and just `DestinationName`
          // see if the name is already in the list
          match = this.services.filterBy('Name', name);
          if (match.length === 0) {
            // if its not make a new 'fake' Service that doesn't exist yet
            // and add it to the possible services to make an intention between
            selected = { Name: name };
            switch (target.name) {
              case 'SourceName':
              case 'DestinationName':
                set(this, 'services', [selected].concat(this.services.toArray()));
                break;
              case 'SourceNS':
              case 'DestinationNS':
                set(this, 'nspaces', [selected].concat(this.nspaces.toArray()));
                break;
            }
          }
          set(this, target.name, selected);
          break;
      }
      form.handleEvent(event);
    },
  },
});
