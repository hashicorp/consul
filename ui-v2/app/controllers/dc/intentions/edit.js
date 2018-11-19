import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
export default Controller.extend({
  dom: service('dom'),
  builder: service('form'),
  init: function() {
    this._super(...arguments);
    this.form = get(this, 'builder').form('intention');
  },
  setProperties: function(model) {
    const sourceName = get(model.item, 'SourceName');
    const destinationName = get(model.item, 'DestinationName');
    let source = model.items.findBy('Name', sourceName);
    let destination = model.items.findBy('Name', destinationName);
    if (!source) {
      source = { Name: sourceName };
      model.items = [source].concat(model.items);
    }
    if (!destination) {
      destination = { Name: destinationName };
      model.items = [destination].concat(model.items);
    }
    this._super({
      ...model,
      ...{
        item: this.form.setData(model.item).getData(),
        SourceName: source,
        DestinationName: destination,
      },
    });
  },
  actions: {
    createNewLabel: function(template, term) {
      return template.replace(/{{term}}/g, term);
    },
    isUnique: function(term) {
      return !get(this, 'items').findBy('Name', term);
    },
    change: function(e, value, item) {
      const event = get(this, 'dom').normalizeEvent(e, value);
      const form = get(this, 'form');
      const target = event.target;

      let name;
      let selected;
      let match;
      switch (target.name) {
        case 'SourceName':
        case 'DestinationName':
          name = selected = target.value;
          // Names can be selected Service EmberObjects or typed in strings
          // if its not a string, use the `Name` from the Service EmberObject
          if (typeof name !== 'string') {
            name = get(target.value, 'Name');
          }
          // see if the name is already in the list
          match = get(this, 'items').filterBy('Name', name);
          if (match.length === 0) {
            // if its not make a new 'fake' Service that doesn't exist yet
            // and add it to the possible services to make an intention between
            selected = { Name: name };
            const items = [selected].concat(this.items.toArray());
            set(this, 'items', items);
          }
          // mutate the value with the string name
          // which will be handled by the form
          target.value = name;
          // these are 'non-form' variables so not on `item`
          // these variables also exist in the template so we know
          // the current selection
          // basically the difference between
          // `item.DestinationName` and just `DestinationName`
          set(this, target.name, selected);
          break;
      }
      form.handleEvent(event);
    },
  },
});
