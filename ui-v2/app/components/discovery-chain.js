import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set, get, computed } from '@ember/object';

const getNodeResolvers = function(nodes = {}) {
  const failovers = getFailovers(nodes);
  const resolvers = {};
  Object.keys(nodes).forEach(function(key) {
    const node = nodes[key];
    if (node.Type === 'resolver' && !failovers.includes(key.split(':').pop())) {
      resolvers[node.Name] = node;
    }
  });
  return resolvers;
};

const getTargetResolvers = function(targets = [], nodes = {}) {
  const resolvers = {};
  Object.values(targets).forEach(item => {
    let node = nodes[item.ID];
    if (node) {
      if (typeof resolvers[item.Service] === 'undefined') {
        resolvers[item.Service] = {
          ID: item.ID,
          Name: item.Service,
          Subsets: [],
          Failover: null,
        };
      }
      const resolver = resolvers[item.Service];
      let failoverable = resolver;
      if (item.ServiceSubset) {
        failoverable = item;
        // FIXME: Sometimes we have set the resolvers ID to the ID of the
        // subset this just shifts the subset of the front of the URL for the moment
        const temp = item.ID.split('.');
        temp.shift();
        resolver.ID = temp.join('.');
        resolver.Subsets.push(item);
      }
      if (typeof node.Resolver.Failover !== 'undefined') {
        failoverable.Failover = targetsToFailover(node.Resolver.Failover.Targets, item.ID);
      }
    }
  });
  return Object.values(resolvers);
};
const getFailovers = function(nodes = {}) {
  const failovers = [];
  Object.values(nodes)
    .filter(item => item.Type === 'resolver')
    .forEach(function(item) {
      (get(item, 'Resolver.Failover.Targets') || []).forEach(failover => {
        failovers.push(failover);
      });
    });
  return failovers;
};
const getType = function(nodes = {}, type) {
  return Object.values(nodes).filter(item => item.Type === type);
};

const targetsToFailover = function(targets, a) {
  let type;
  const Targets = targets.map(function(b) {
    // FIXME: this isn't going to work past namespace for services
    // with dots in the name
    const [aRev, bRev] = [a, b].map(item => item.split('.').reverse());
    const types = ['Datacenter', 'Namespace', 'Service', 'Subset'];
    return bRev.find(function(item, i) {
      const res = item !== aRev[i];
      if (res) {
        type = types[i];
      }
      return res;
    });
  });
  return {
    Type: type,
    Targets: Targets,
  };
};

export default Component.extend({
  dom: service('dom'),
  dataStructs: service('data-structs'),
  classNames: ['discovery-chain'],
  classNameBindings: ['active'],
  selectedId: '',
  x: 0,
  y: 0,
  tooltip: '',
  activeTooltip: false,
  splitters: computed('chain.Nodes', function() {
    return getType(get(this, 'chain.Nodes'), 'splitter').map(function(item) {
      set(item, 'ID', `splitter:${item.Name}`);
      return item;
    });
  }),
  routers: computed('chain.Nodes', function() {
    // Right now there should only ever be one 'Router'.
    return getType(get(this, 'chain.Nodes'), 'router');
  }),
  routes: computed('routers', function() {
    return get(this, 'routers').reduce(function(prev, item) {
      return prev.concat(
        item.Routes.map(function(route, i) {
          return {
            ...route,
            ID: `route:${item.Name}-${i}`,
          };
        })
      );
    }, []);
  }),
  nodeResolvers: computed('chain.Nodes', function() {
    return getNodeResolvers(get(this, 'chain.Nodes'));
  }),
  resolvers: computed('nodeResolvers.[]', 'chain.Targets', function() {
    return getTargetResolvers(get(this, 'chain.Targets'), this.nodeResolvers);
  }),
  graph: computed('chain.Nodes', function() {
    const graph = this.dataStructs.graph();
    Object.entries(get(this, 'chain.Nodes')).forEach(function([key, item]) {
      switch (item.Type) {
        case 'splitter':
          item.Splits.forEach(function(splitter) {
            graph.addLink(`splitter:${item.Name}`, splitter.NextNode);
          });
          break;
        case 'router':
          item.Routes.forEach(function(route, i) {
            graph.addLink(`route:${item.Name}-${i}`, route.NextNode);
          });
          break;
      }
    });
    return graph;
  }),
  selected: computed('selectedId', 'graph', function() {
    if (this.selectedId === '' || !this.dom.element(`#${this.selectedId}`)) {
      return {};
    }
    const getTypeFromId = function(id) {
      return id.split(':').shift();
    };
    const id = this.selectedId;
    const type = getTypeFromId(id);
    const nodes = [id];
    const edges = [];
    this.graph.forEachLinkedNode(id, (linkedNode, link) => {
      nodes.push(linkedNode.id);
      edges.push(`${link.fromId}>${link.toId}`);
      this.graph.forEachLinkedNode(linkedNode.id, (linkedNode, link) => {
        const nodeType = getTypeFromId(linkedNode.id);
        if (type !== nodeType && type !== 'splitter' && nodeType !== 'splitter') {
          nodes.push(linkedNode.id);
          edges.push(`${link.fromId}>${link.toId}`);
        }
      });
    });
    // FIXME: Use https://github.com/mathiasbynens/CSS.escape
    return {
      nodes: nodes.map(item => `#${CSS.escape(item)}`),
      edges: edges.map(item => `#${CSS.escape(item)}`),
    };
  }),
  width: computed('chain.{Nodes,Targets}', function() {
    return this.element.offsetWidth;
  }),
  height: computed('chain.{Nodes,Targets}', function() {
    return this.element.offsetHeight;
  }),
  didInsertElement: function() {
    this.addPathListeners();
  },
  didReceiveAttrs: function() {
    if (this.element) {
      this.addPathListeners();
    }
  },
  // TODO(octane): ember has trouble adding mouse events to svg elements whilst giving
  // the developer access to the mouse event therefore we just use JS to add our events
  // revisit this post Octane
  addPathListeners: function() {
    if (typeof this._listeners === 'undefined') {
      this._listeners = this.dom.listeners();
    }
    this._listeners.remove();
    [...this.dom.elements('path.split', this.element)].forEach(item => {
      this._listeners.add(item, {
        mouseover: e => this.actions.showSplit.apply(this, [e]),
        mouseout: e => this.actions.hideSplit.apply(this, [e]),
      });
    });
    // TODO: currently don't think there is a way to listen
    // for an element being removed inside a component, possibly
    // using IntersectionObserver. It's a tiny detail, but we just always
    // remove the tooltip on component update as its so tiny, ideal
    // the tooltip would stay if there was no change to the <path>
    set(this, 'activeTooltip', false);
  },
  willDestroyElement: function() {
    this._listeners.remove();
  },
  actions: {
    showSplit: function(e) {
      this.setProperties({
        x: e.offsetX,
        y: e.offsetY - 5,
        tooltip: e.target.dataset.percentage,
        activeTooltip: true,
      });
    },
    hideSplit: function(e = null) {
      set(this, 'activeTooltip', false);
    },
    click: function(e) {
      const id = e.currentTarget.getAttribute('id');
      if (id === this.selectedId) {
        set(this, 'active', false);
        set(this, 'selectedId', '');
      } else {
        set(this, 'active', true);
        set(this, 'selectedId', id);
      }
    },
  },
});
