import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { setProperties, set, get } from '@ember/object';
import { assert } from '@ember/debug';

export default Component.extend({
  tagName: '',
  dom: service('dom'),
  builder: service('form'),
  init: function() {
    this._super(...arguments);
    this.form = this.builder.form('intention');
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    if (this.item && this.services && this.nspaces) {
      let services = this.services || [];
      let nspaces = this.nspaces || [];
      let source = services.findBy('Name', this.item.SourceName);
      if (!source) {
        source = { Name: this.item.SourceName };
        services = [source].concat(services);
      }
      let destination = services.findBy('Name', this.item.DestinationName);
      if (!destination) {
        destination = { Name: this.item.DestinationName };
        services = [destination].concat(services);
      }

      let sourceNS = nspaces.findBy('Name', this.item.SourceNS);
      if (!sourceNS) {
        sourceNS = { Name: this.item.SourceNS };
        nspaces = [sourceNS].concat(nspaces);
      }
      let destinationNS = this.nspaces.findBy('Name', this.item.DestinationNS);
      if (!destinationNS) {
        destinationNS = { Name: this.item.DestinationNS };
        nspaces = [destinationNS].concat(nspaces);
      }
      // TODO: Use this.{item,services} when we have this.args
      setProperties(this, {
        _item: this.form.setData(this.item).getData(),
        _services: services,
        _nspaces: nspaces,
        SourceName: source,
        DestinationName: destination,
        SourceNS: sourceNS,
        DestinationNS: destinationNS,
      });
    } else {
      assert('@item, @services and @nspaces are required arguments', false);
    }
  },
  actions: {
    createNewLabel: function(template, term) {
      return template.replace(/{{term}}/g, term);
    },
    isUnique: function(term) {
      return !this._services.findBy('Name', term);
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
          match = this._services.filterBy('Name', name);
          if (match.length === 0) {
            // if its not make a new 'fake' Service that doesn't exist yet
            // and add it to the possible services to make an intention between
            selected = { Name: name };
            switch (target.name) {
              case 'SourceName':
              case 'DestinationName':
                set(this, '_services', [selected].concat(this._services.toArray()));
                break;
              case 'SourceNS':
              case 'DestinationNS':
                set(this, '_nspaces', [selected].concat(this._nspaces.toArray()));
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
