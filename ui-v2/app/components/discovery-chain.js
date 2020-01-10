import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set, get, computed } from '@ember/object';
import { next } from '@ember/runloop';

export const getNodesByType = function(nodes = {}, type) {
  return Object.values(nodes).filter(item => item.Type === type);
};
export const getSplitters = function(nodes) {
  return getNodesByType(nodes, 'splitter').map(function(item) {
    // Splitters need IDs adding so we can find them in the DOM later
    item.ID = `splitter:${item.Name}`;
    return item;
  });
};
export const getRoutes = function(nodes) {
  return getNodesByType(nodes, 'router').reduce(function(prev, item) {
    return prev.concat(
      item.Routes.map(function(route, i) {
        // Routes also have IDs added via createRoute
        return createRoute(route, item.Name);
      })
    );
  }, []);
};
export const findResolver = function(resolvers, service, nspace, dc) {
  if (typeof resolvers[service] === 'undefined') {
    resolvers[service] = {
      ID: `${service}.${nspace}.${dc}`,
      Name: service,
      Children: [],
    };
  }
  return resolvers[service];
};
export const getResolvers = function(dc, nspace = 'default', targets = [], nodes = {}) {
  const resolvers = {};
  Object.values(targets).forEach(target => {
    const node = nodes[`resolver:${target.ID}`];
    const resolver = findResolver(resolvers, target.Service, nspace, dc);
    // We use this to figure out whether this target is a redirect target
    const alternate = getAlternateServices([target.ID], `service.${nspace}.${dc}`);

    let failovers;
    // Figure out the failover type
    if (typeof node.Resolver.Failover !== 'undefined') {
      failovers = getAlternateServices(node.Resolver.Failover.Targets, target.ID);
    }
    switch (true) {
      // This target is a redirect
      case alternate.Type !== 'Service':
        resolver.Children.push({
          Redirect: true,
          ID: target.ID,
          Name: target[alternate.Type],
        });
        break;
      // This target is a Subset
      case typeof target.ServiceSubset !== 'undefined':
        resolver.Children.push({
          Subset: true,
          ID: target.ID,
          Name: target.ServiceSubset,
          Filter: target.Subset.Filter,
          Failover: failovers,
        });
        break;
      // This target is just normal service that might have failovers
      default:
        resolver.Failover = failovers;
    }
  });
  return Object.values(resolvers);
};
export const getAlternateServices = function(targets, a) {
  let type;
  const Targets = targets.map(function(b) {
    // TODO: this isn't going to work past namespace for services
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

export const createRoute = function(route, router) {
  let id;
  if (typeof route.Definition.Match === 'undefined') {
    id = 'route:default';
    route.Default = true;
  } else {
    id = `route:${router}-${JSON.stringify(route.Definition)}`;
  }
  return {
    ...route,
    ID: id,
  };
};
export default Component.extend({
  dom: service('dom'),
  ticker: service('ticker'),
  dataStructs: service('data-structs'),
  classNames: ['discovery-chain'],
  classNameBindings: ['active'],
  isDisplayed: false,
  selectedId: '',
  x: 0,
  y: 0,
  tooltip: '',
  activeTooltip: false,
  init: function() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
    this._viewportlistener = this.dom.listeners();
  },
  didInsertElement: function() {
    this._super(...arguments);
    this._viewportlistener.add(
      this.dom.isInViewport(this.element, bool => {
        set(this, 'isDisplayed', bool);
        if (this.isDisplayed) {
          this.addPathListeners();
        } else {
          this.ticker.destroy(this);
        }
      })
    );
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    if (this.element) {
      this.addPathListeners();
    }
  },
  willDestroyElement: function() {
    this._super(...arguments);
    this._listeners.remove();
    this._viewportlistener.remove();
    this.ticker.destroy(this);
  },
  splitters: computed('chain.Nodes', function() {
    return getSplitters(get(this, 'chain.Nodes'));
  }),
  routes: computed('chain.Nodes', function() {
    return getRoutes(get(this, 'chain.Nodes'));
  }),
  resolvers: computed('chain.{Nodes,Targets}', function() {
    return getResolvers(
      this.chain.Datacenter,
      this.chain.Namespace,
      get(this, 'chain.Targets'),
      get(this, 'chain.Nodes')
    );
  }),
  graph: computed('chain.Nodes', function() {
    const graph = this.dataStructs.graph();
    const router = this.chain.ServiceName;
    Object.entries(get(this, 'chain.Nodes')).forEach(function([key, item]) {
      switch (item.Type) {
        case 'splitter':
          item.Splits.forEach(function(splitter) {
            graph.addLink(`splitter:${item.Name}`, splitter.NextNode);
          });
          break;
        case 'router':
          item.Routes.forEach(function(route, i) {
            route = createRoute(route, router);
            graph.addLink(route.ID, route.NextNode);
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
    return {
      nodes: nodes.map(item => `#${CSS.escape(item)}`),
      edges: edges.map(item => `#${CSS.escape(item)}`),
    };
  }),
  width: computed('isDisplayed', 'chain.{Nodes,Targets}', function() {
    return this.element.offsetWidth;
  }),
  height: computed('isDisplayed', 'chain.{Nodes,Targets}', function() {
    return this.element.offsetHeight;
  }),
  // TODO(octane): ember has trouble adding mouse events to svg elements whilst giving
  // the developer access to the mouse event therefore we just use JS to add our events
  // revisit this post Octane
  addPathListeners: function() {
    // TODO: Figure out if we can remove this next
    next(() => {
      this._listeners.remove();
      [...this.dom.elements('path.split', this.element)].forEach(item => {
        this._listeners.add(item, {
          mouseover: e => this.actions.showSplit.apply(this, [e]),
          mouseout: e => this.actions.hideSplit.apply(this, [e]),
        });
      });
    });
    // TODO: currently don't think there is a way to listen
    // for an element being removed inside a component, possibly
    // using IntersectionObserver. It's a tiny detail, but we just always
    // remove the tooltip on component update as its so tiny, ideal
    // the tooltip would stay if there was no change to the <path>
    // set(this, 'activeTooltip', false);
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
