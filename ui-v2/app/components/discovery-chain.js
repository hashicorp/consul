import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set, get, computed } from '@ember/object';
import { next } from '@ember/runloop';

import {
  createRoute,
  getSplitters,
  getRoutes,
  getResolvers,
} from 'consul-ui/utils/components/discovery-chain/index';

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
    return getRoutes(get(this, 'chain.Nodes'), this.dom.guid);
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
    Object.entries(get(this, 'chain.Nodes')).forEach(([key, item]) => {
      switch (item.Type) {
        case 'splitter':
          item.Splits.forEach(splitter => {
            graph.addLink(`splitter:${item.Name}`, splitter.NextNode);
          });
          break;
        case 'router':
          item.Routes.forEach((route, i) => {
            route = createRoute(route, router, this.dom.guid);
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
    // TODO: Figure out if we can remove this next
    next(() => {
      this._listeners.remove();
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
