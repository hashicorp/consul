import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
export default Controller.extend({
  dom: service('dom'),
  builder: service('form'),
  init: function() {
    this._super(...arguments);
    this.form = this.builder.form('intention');
  },
  setProperties: function(model) {
    let source = model.services.findBy('Name', model.item.SourceName);

    if (!source) {
      source = { Name: model.item.SourceName };
      model.services = [source].concat(model.services);
    }
    let destination = model.services.findBy('Name', model.item.DestinationName);
    if (!destination) {
      destination = { Name: model.item.DestinationName };
      model.services = [destination].concat(model.services);
    }

    let sourceNS = model.nspaces.findBy('Name', model.item.SourceNS);
    if (!sourceNS) {
      sourceNS = { Name: model.item.SourceNS };
      model.nspaces = [sourceNS].concat(model.nspaces);
    }
    let destinationNS = model.nspaces.findBy('Name', model.item.DestinationNS);
    if (!destinationNS) {
      destinationNS = { Name: model.item.DestinationNS };
      model.nspaces = [destinationNS].concat(model.nspaces);
    }
    this._super({
      ...model,
      ...{
        item: this.form.setData(model.item).getData(),
        SourceName: source,
        DestinationName: destination,
        SourceNS: sourceNS,
        DestinationNS: destinationNS,
      },
    });
  },
  actions: {
    createNewLabel: function(template, term) {
      return template.replace(/{{term}}/g, term);
    },
    isUnique: function(term) {
      return !this.services.findBy('Name', term);
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
