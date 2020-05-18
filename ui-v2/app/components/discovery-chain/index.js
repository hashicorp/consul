import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set, get, computed } from '@ember/object';
import { schedule } from '@ember/runloop';

import { createRoute, getSplitters, getRoutes, getResolvers } from './utils';

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
        if (get(this, 'isDisplayed') !== bool) {
          set(this, 'isDisplayed', bool);
          if (this.isDisplayed) {
            this.addPathListeners();
          } else {
            this.ticker.destroy(this);
          }
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
    const routes = getRoutes(get(this, 'chain.Nodes'), this.dom.guid);
    // if we have no routes with a PathPrefix of '/' or one with no definition at all
    // then add our own 'default catch all'
    if (
      !routes.find(item => get(item, 'Definition.Match.HTTP.PathPrefix') === '/') &&
      !routes.find(item => typeof item.Definition === 'undefined')
    ) {
      let nextNode;
      const resolverID = `resolver:${this.chain.ServiceName}.${this.chain.Namespace}.${this.chain.Datacenter}`;
      const splitterID = `splitter:${this.chain.ServiceName}.${this.chain.Namespace}`;
      // The default router should look for a splitter first,
      // if there isn't one try the default resolver
      if (typeof this.chain.Nodes[splitterID] !== 'undefined') {
        nextNode = splitterID;
      } else if (typeof this.chain.Nodes[resolverID] !== 'undefined') {
        nextNode = resolverID;
      }
      if (typeof nextNode !== 'undefined') {
        const route = {
          Default: true,
          ID: `route:${this.chain.ServiceName}`,
          Name: this.chain.ServiceName,
          Definition: {
            Match: {
              HTTP: {
                PathPrefix: '/',
              },
            },
          },
          NextNode: nextNode,
        };
        routes.push(createRoute(route, this.chain.ServiceName, this.dom.guid));
      }
    }
    return routes;
  }),
  resolvers: computed('chain.{Nodes,Targets}', function() {
    return getResolvers(
      this.chain.Datacenter,
      this.chain.Namespace,
      get(this, 'chain.Targets'),
      get(this, 'chain.Nodes')
    );
  }),
  graph: computed('splitters', 'routes.[]', function() {
    const graph = this.dataStructs.graph();
    this.splitters.forEach(item => {
      item.Splits.forEach(splitter => {
        graph.addLink(item.ID, splitter.NextNode);
      });
    });
    this.routes.forEach((route, i) => {
      graph.addLink(route.ID, route.NextNode);
    });
    return graph;
  }),
  selected: computed('selectedId', 'graph', function() {
    if (this.selectedId === '' || !this.dom.element(`#${this.selectedId}`)) {
      return {};
    }
    const id = this.selectedId;
    const type = id.split(':').shift();
    const nodes = [id];
    const edges = [];
    this.graph.forEachLinkedNode(id, (linkedNode, link) => {
      nodes.push(linkedNode.id);
      edges.push(`${link.fromId}>${link.toId}`);
      this.graph.forEachLinkedNode(linkedNode.id, (linkedNode, link) => {
        const nodeType = linkedNode.id.split(':').shift();
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
    schedule('afterRender', () => {
      this._listeners.remove();
      // as this is now afterRender, theoretically
      // it could happen after the component is destroyed?
      // watch for that incase
      if (this.element && !this.isDestroyed) {
        this._listeners.add(this.dom.document(), {
          click: e => {
            // all route/splitter/resolver components currently
            // have classes that end in '-card'
            if (!this.dom.closest('[class$="-card"]', e.target)) {
              set(this, 'active', false);
              set(this, 'selectedId', '');
            }
          },
        });
        [...this.dom.elements('path.split', this.element)].forEach(item => {
          this._listeners.add(item, {
            mouseover: e => this.actions.showSplit.apply(this, [e]),
            mouseout: e => this.actions.hideSplit.apply(this, [e]),
          });
        });
      }
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
