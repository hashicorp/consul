/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set, get, computed } from '@ember/object';

import { createRoute, getSplitters, getRoutes, getResolvers } from './utils';

export default Component.extend({
  dom: service('dom'),
  ticker: service('ticker'),
  dataStructs: service('data-structs'),
  classNames: ['discovery-chain'],
  classNameBindings: ['active'],
  selectedId: '',
  init: function () {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
  },
  didInsertElement: function () {
    this._listeners.add(this.dom.document(), {
      click: (e) => {
        // all route/splitter/resolver components currently
        // have classes that end in '-card'
        if (!this.dom.closest('[class$="-card"]', e.target)) {
          set(this, 'active', false);
          set(this, 'selectedId', '');
        }
      },
    });
  },
  willDestroyElement: function () {
    this._super(...arguments);
    this._listeners.remove();
    this.ticker.destroy(this);
  },
  splitters: computed('chain.Nodes', function () {
    return getSplitters(get(this, 'chain.Nodes'));
  }),
  routes: computed('chain.Nodes', function () {
    const routes = getRoutes(get(this, 'chain.Nodes'), this.dom.guid);
    // if we have no routes with a PathPrefix of '/' or one with no definition at all
    // then add our own 'default catch all'
    if (
      !routes.find((item) => get(item, 'Definition.Match.HTTP.PathPrefix') === '/') &&
      !routes.find((item) => typeof item.Definition === 'undefined')
    ) {
      let nextNode;
      const resolverID = `resolver:${this.chain.ServiceName}.${this.chain.Namespace}.${this.chain.Partition}.${this.chain.Datacenter}`;
      const splitterID = `splitter:${this.chain.ServiceName}.${this.chain.Namespace}.${this.chain.Partition}`;
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
  nodes: computed('routes', 'splitters', 'resolvers', function () {
    let nodes = this.resolvers.reduce((prev, item) => {
      prev[`resolver:${item.ID}`] = item;
      item.Children.reduce((prev, item) => {
        prev[`resolver:${item.ID}`] = item;
        return prev;
      }, prev);
      return prev;
    }, {});
    nodes = this.splitters.reduce((prev, item) => {
      prev[item.ID] = item;
      return prev;
    }, nodes);
    nodes = this.routes.reduce((prev, item) => {
      prev[item.ID] = item;
      return prev;
    }, nodes);
    Object.entries(nodes).forEach(([key, value]) => {
      if (typeof value.NextNode !== 'undefined') {
        value.NextItem = nodes[value.NextNode];
      }
      if (typeof value.Splits !== 'undefined') {
        value.Splits.forEach((item) => {
          if (typeof item.NextNode !== 'undefined') {
            item.NextItem = nodes[item.NextNode];
          }
        });
      }
    });
    return '';
  }),
  resolvers: computed('chain.{Nodes,Targets}', function () {
    return getResolvers(
      this.chain.Datacenter,
      this.chain.Partition,
      this.chain.Namespace,
      get(this, 'chain.Targets'),
      get(this, 'chain.Nodes')
    );
  }),
  graph: computed('splitters', 'routes.[]', function () {
    const graph = this.dataStructs.graph();
    this.splitters.forEach((item) => {
      item.Splits.forEach((splitter) => {
        graph.addLink(item.ID, splitter.NextNode);
      });
    });
    this.routes.forEach((route, i) => {
      graph.addLink(route.ID, route.NextNode);
    });
    return graph;
  }),
  selected: computed('selectedId', 'graph', function () {
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
      nodes: nodes.map((item) => `#${CSS.escape(item)}`),
      edges: edges.map((item) => `#${CSS.escape(item)}`),
    };
  }),
  actions: {
    click: function (e) {
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
