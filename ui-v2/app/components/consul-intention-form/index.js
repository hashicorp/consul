import Component from '@ember/component';
import { setProperties, set, get } from '@ember/object';

export default Component.extend({
  tagName: '',
  ondelete: function() {
    this.onsubmit(...arguments);
  },
  oncancel: function() {
    this.onsubmit(...arguments);
  },
  onsubmit: function() {},
  actions: {
    createServices: function(item, e) {
      // Services in the menus should:
      // 1. Be unique (they potentially could be duplicated due to services from different namespaces)
      // 2. Only include services that shold have intentions
      // 3. Include an 'All Services' option
      // 4. Include the current Source and Destination incase they are virtual services/don't exist yet
      let items = e.data
        .uniqBy('Name')
        .toArray()
        .filter(
          item => !['connect-proxy', 'mesh-gateway', 'terminating-gateway'].includes(item.Kind)
        )
        .sort((a, b) => a.Name.localeCompare(b.Name));
      items = [{ Name: '*' }].concat(items);
      let source = items.findBy('Name', item.SourceName);
      if (!source) {
        source = { Name: item.SourceName };
        items = [source].concat(items);
      }
      let destination = items.findBy('Name', item.DestinationName);
      if (!destination) {
        destination = { Name: item.DestinationName };
        items = [destination].concat(items);
      }
      setProperties(this, {
        services: items,
        SourceName: source,
        DestinationName: destination,
      });
    },
    createNspaces: function(item, e) {
      // Nspaces in the menus should:
      // 1. Include an 'All Namespaces' option
      // 2. Include the current SourceNS and DestinationNS incase they don't exist yet
      let items = e.data.toArray().sort((a, b) => a.Name.localeCompare(b.Name));
      items = [{ Name: '*' }].concat(items);
      let source = items.findBy('Name', item.SourceNS);
      if (!source) {
        source = { Name: item.SourceNS };
        items = [source].concat(items);
      }
      let destination = items.findBy('Name', item.DestinationNS);
      if (!destination) {
        destination = { Name: item.DestinationNS };
        items = [destination].concat(items);
      }
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
    change: function(e, form, item) {
      const target = e.target;

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
      form.handleEvent(e);
    },
  },
});
