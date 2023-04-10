/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import EventTarget from 'consul-ui/utils/dom/event-target/rsvp';
import { set } from '@ember/object';
const IntervalTickerGroup = class extends EventTarget {
  constructor(rate = 1000 / 60) {
    super();
    this.setRate(rate);
  }
  tick() {
    this.dispatchEvent({ type: 'tick', target: this });
  }
  setRate(rate) {
    clearInterval(this._interval);
    this._interval = setInterval(() => this.tick(), rate);
  }
  destroy() {
    clearInterval(this._interval);
  }
};
export const Ticker = class extends EventTarget {
  static destroy() {
    if (typeof Ticker.defaultTickerGroup !== 'undefined') {
      Ticker.defaultTickerGroup.destroy();
      delete Ticker.defaultTickerGroup;
    }
  }
  constructor(obj) {
    super();
    this.setTickable(obj);
  }
  tick() {
    this._tickable.tick();
  }
  setTickable(tickable) {
    this._tickable = tickable;
    // this.addEventListener(this._tickable);
    if (typeof this._tickable.getTicker === 'undefined') {
      this._tickable.getTicker = () => this;
    }
    this.tick = this._tickable.tick.bind(this._tickable);
  }
  getTickable() {
    return this._tickable;
  }
  isAlive() {
    return this._isAlive;
  }
  start() {
    this._isAlive = true;
    this.getTickerGroup().addEventListener('tick', this.tick);
    this.dispatchEvent({ type: 'start', target: this });
  }
  stop() {
    this._isAlive = false;
    this.getTickerGroup().removeEventListener('tick', this.tick);
    this.dispatchEvent({ type: 'stop', target: this });
  }
  activeCount() {
    return this.getTickerGroup().activeCount();
  }
  setTickerGroup(group) {
    this._group = group;
  }
  getTickerGroup() {
    if (typeof this._group === 'undefined') {
      if (typeof Ticker.defaultTickerGroup === 'undefined') {
        Ticker.defaultTickerGroup = new TickerGroup();
      }
      this._group = Ticker.defaultTickerGroup;
    }
    return this._group;
  }
};
const TimelineAbstract = class {
  constructor() {
    this._currentframe = 1;
    this.setIncrement(1);
  }
  isAtStart() {
    return this._currentframe <= 1;
  }
  isAtEnd() {
    return this._currentframe >= this._totalframes;
  }
  addEventListener() {
    return this.getTicker().addEventListener(...arguments);
  }
  removeEventListener() {
    return this.getTicker().removeEventListener(...arguments);
  }
  stop() {
    return this.gotoAndStop(this._currentframe);
  }
  play() {
    return this.gotoAndPlay(this._currentframe);
  }
  start() {
    return this.gotoAndPlay(this._currentframe);
  }
  gotoAndStop(frame) {
    this._currentframe = frame;
    const ticker = this.getTicker();
    if (ticker.isAlive()) {
      ticker.stop();
    }
    return this;
  }
  gotoAndPlay(frame) {
    this._currentframe = frame;
    const ticker = this.getTicker();
    if (!ticker.isAlive()) {
      ticker.start();
    }
    return this;
  }
  getTicker() {
    if (typeof this._ticker === 'undefined') {
      this._ticker = new Ticker(this);
    }
    return this._ticker;
  }
  setFrames(frames) {
    this._totalframes = frames;
    return this;
  }
  setIncrement(inc) {
    this._increment = inc;
    return this;
  }
};
const Cubic = {
  easeOut: function (t, b, c, d) {
    t /= d;
    t--;
    return c * (t * t * t + 1) + b;
  },
};
const TickerGroup = IntervalTickerGroup;
export const Tween = class extends TimelineAbstract {
  static destroy() {
    Ticker.destroy();
  }
  static to(start, finish, frames, method) {
    Object.keys(finish).forEach(function (key) {
      finish[key] -= start[key];
    });
    return new Tween(start, finish, frames, method).play();
  }
  constructor(obj, props, frames = 12, method = Cubic.easeOut) {
    super();
    this.setMethod(method);
    this.setProps(props);
    this.setTarget(obj);
    this.setFrames(frames);
    this.tick = this.forwards;
  }
  _process() {
    Object.keys(this._props).forEach((key) => {
      const num = this._method(
        this._currentframe,
        this._initialstate[key],
        this._props[key],
        this._totalframes
      );
      // this._target[key] = num;
      set(this._target, key, num);
    });
  }
  forwards() {
    if (this._currentframe <= this._totalframes) {
      this._process();
      this._currentframe += this._increment;
    } else {
      this._currentframe = this._totalframes;
      this.getTicker().stop();
    }
  }
  backwards() {
    this._currentframe -= this._increment;
    if (this._currentframe >= 0) {
      this._process();
    } else {
      this.run = this.forwards;
      this._currentframe = 1;
      this.getTicker().stop();
    }
  }
  gotoAndPlay() {
    if (typeof this._initialstate === 'undefined') {
      this._initialstate = {};
      Object.keys(this._props).forEach((key) => {
        this._initialstate[key] = this._target[key];
      });
    }
    return super.gotoAndPlay(...arguments);
  }
  setTarget(target) {
    this._target = target;
  }
  getTarget(target) {
    return this._target;
  }
  setProps(props) {
    this._props = props;
    return this;
  }
  setMethod(method) {
    this._method = method;
  }
};
