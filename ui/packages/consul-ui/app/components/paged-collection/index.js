/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { scheduleOnce } from '@ember/runloop';

export default class PagedCollectionComponent extends Component {
  @tracked $pane;
  @tracked $viewport;

  @tracked top = 0;
  @tracked visibleItems = 0;
  @tracked overflow = 10;
  @tracked _rowHeight = 0;

  @tracked _type = 'native-scroll';

  get type() {
    return this.args.type || this._type;
  }

  get items() {
    return this.args.items.slice(this.cursor, this.cursor + this.perPage);
  }

  get perPage() {
    switch (this.type) {
      case 'virtual-scroll':
        return this.visibleItems + this.overflow * 2;
      case 'index':
        return parseInt(this.args.perPage);
    }
    // 'native-scroll':
    return this.total;
  }

  get cursor() {
    switch (this.type) {
      case 'virtual-scroll':
        return this.itemsBefore;
      case 'index':
        return (parseInt(this.args.page) - 1) * this.perPage;
    }
    // 'native-scroll':
    return 0;
  }

  get itemsBefore() {
    if (typeof this.$viewport === 'undefined') {
      return 0;
    }
    return Math.max(0, Math.round(this.top / this.rowHeight) - this.overflow);
  }

  get rowHeight() {
    return parseFloat(this.args.rowHeight || this._rowHeight);
  }

  get startHeight() {
    switch (this.type) {
      case 'virtual-scroll':
        return Math.min(this.totalHeight, this.itemsBefore * this.rowHeight);
      case 'index':
        return 0;
    }
    // 'native-scroll':
    return 0;
  }

  get totalHeight() {
    return this.total * this.rowHeight;
  }
  get totalPages() {
    return Math.ceil(this.total / this.perPage);
  }

  get total() {
    return this.args.items.length;
  }

  @action
  scroll(e) {
    this.top = this.$viewport.scrollTop;
  }

  @action
  resize() {
    if (this.$viewport.clientHeight > 0 && this.rowHeight > 0) {
      this.visibleItems = Math.ceil(this.$viewport.clientHeight / this.rowHeight);
    } else {
      this.visibleItems = 0;
    }
  }

  @action
  setViewport($viewport) {
    this.$viewport =
      $viewport === 'html' ? [...document.getElementsByTagName('html')][0] : $viewport;
    this.$viewport.addEventListener('scroll', this.scroll);
    if ($viewport === 'html') {
      this.$viewport.addEventListener('resize', this.resize);
    }
    this.scroll();
    this.resize();
  }

  @action setPane($pane) {
    this.$pane = $pane;
  }

  @action setRowHeight(str) {
    this._rowHeight = parseFloat(str);
  }

  @action setMaxHeight(str) {
    scheduleOnce('actions', this, '_setMaxHeight');
  }

  @action _setMaxHeight(str) {
    const maxHeight = parseFloat(str);

    if (!isNaN(maxHeight)) {
      this._type = 'virtual-scroll';
    }
  }

  @action
  disconnect() {
    this.$viewport.removeEventListener('scroll', this.scroll);
    this.$viewport.removeEventListener('resize', this.resize);
  }
}
