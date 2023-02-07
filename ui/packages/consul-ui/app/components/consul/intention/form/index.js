import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class ConsulIntentionForm extends Component {
  @tracked services;
  @tracked SourceName;
  @tracked DestinationName;

  @tracked nspaces;
  @tracked SourceNS;
  @tracked DestinationNS;

  @tracked partitions;
  @tracked SourcePartition;
  @tracked DestinationPartition;

  @tracked isManagedByCRDs;

  modal = null; // reference to the warning modal

  @service('repository/intention') repo;

  constructor(owner, args) {
    super(...arguments);
    this.updateCRDManagement();
  }

  @action
  ondelete() {
    if (this.args.ondelete) {
      this.args.ondelete(...arguments);
    } else {
      this.onsubmit(...arguments);
    }
  }

  @action
  oncancel() {
    if (this.args.oncancel) {
      this.args.oncancel(...arguments);
    } else {
      this.onsubmit(...arguments);
    }
  }

  @action
  onsubmit() {
    if (this.args.onsubmit) {
      this.args.onsubmit(...arguments);
    }
  }

  @action
  updateCRDManagement() {
    this.isManagedByCRDs = this.repo.isManagedByCRDs();
  }

  @action
  submit(item, submit, e) {
    e.preventDefault();
    // if the action of the intention has changed and its non-empty then warn
    // the user
    if (typeof item.change.Action !== 'undefined' && typeof item.data.Action === 'undefined') {
      this.modal.open();
    } else {
      submit();
    }
  }

  @action
  createServices(item, e) {
    // Services in the menus should:
    // 1. Be unique (they potentially could be duplicated due to services from different namespaces)
    // 2. Only include services that shold have intentions
    // 3. Include an 'All Services' option
    // 4. Include the current Source and Destination incase they are virtual services/don't exist yet
    let items = e.data
      .uniqBy('Name')
      .toArray()
      .filter(
        (item) => !['connect-proxy', 'mesh-gateway', 'terminating-gateway'].includes(item.Kind)
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
    this.services = items;
    this.SourceName = source;
    this.DestinationName = destination;
  }

  @action
  createNspaces(item, e) {
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
    this.nspaces = items;
    this.SourceNS = source;
    this.DestinationNS = destination;
  }

  @action
  createPartitions(item, e) {
    // Partitions in the menus should:
    // 1. NOT include an 'All Partitions' option
    // 2. Include the current SourcePartition and DestinationPartition incase they don't exist yet
    let items = e.data.toArray().sort((a, b) => a.Name.localeCompare(b.Name));
    let source = items.findBy('Name', item.SourcePartition);
    if (!source) {
      source = { Name: item.SourcePartition };
      items = [source].concat(items);
    }
    let destination = items.findBy('Name', item.DestinationPartition);
    if (!destination) {
      destination = { Name: item.DestinationPartition };
      items = [destination].concat(items);
    }
    this.partitions = items;
    this.SourcePartition = source;
    this.DestinationPartition = destination;
  }

  @action
  change(e, form, item) {
    const target = e.target;

    let name, selected;
    switch (target.name) {
      case 'SourceName':
      case 'DestinationName':
      case 'SourceNS':
      case 'DestinationNS':
      case 'SourcePartition':
      case 'DestinationPartition':
        name = selected = target.value;
        // Names can be selected Service EmberObjects or typed in strings
        // if its not a string, use the `Name` from the Service EmberObject
        if (typeof name !== 'string') {
          name = target.value.Name;
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

        // if its not make a new 'fake' Service that doesn't exist yet
        // and add it to the possible services to make an intention between
        switch (target.name) {
          case 'SourceName':
          case 'DestinationName':
            if (this.services.filterBy('Name', name).length === 0) {
              selected = { Name: name };
              this.services = [selected].concat(this.services.toArray());
            }
            break;
          case 'SourceNS':
          case 'DestinationNS':
            if (this.nspaces.filterBy('Name', name).length === 0) {
              selected = { Name: name };
              this.nspaces = [selected].concat(this.nspaces.toArray());
            }
            break;
          case 'SourcePartition':
          case 'DestinationPartition':
            if (this.partitions.filterBy('Name', name).length === 0) {
              selected = { Name: name };
              this.partitions = [selected].concat(this.partitions.toArray());
            }
            break;
        }
        this[target.name] = selected;
        break;
    }
    form.handleEvent(e);
  }
}
