import ObjectProxy from '@ember/object/proxy';
import ArrayProxy from '@ember/array/proxy';
import { Promise } from 'rsvp';

import createListeners from 'consul-ui/utils/dom/create-listeners';

import EventTarget from 'consul-ui/utils/dom/event-target/rsvp';

import cacheFactory from 'consul-ui/utils/dom/event-source/cache';
import proxyFactory from 'consul-ui/utils/dom/event-source/proxy';
import firstResolverFactory from 'consul-ui/utils/dom/event-source/resolver';

import CallableEventSourceFactory from 'consul-ui/utils/dom/event-source/callable';
import ReopenableEventSourceFactory from 'consul-ui/utils/dom/event-source/reopenable';
import BlockingEventSourceFactory from 'consul-ui/utils/dom/event-source/blocking';
import StorageEventSourceFactory from 'consul-ui/utils/dom/event-source/storage';

// All The EventSource-i
export const CallableEventSource = CallableEventSourceFactory(EventTarget, Promise);
export const ReopenableEventSource = ReopenableEventSourceFactory(CallableEventSource);
export const BlockingEventSource = BlockingEventSourceFactory(ReopenableEventSource);
export const StorageEventSource = StorageEventSourceFactory(EventTarget, Promise);

// various utils
export const proxy = proxyFactory(ObjectProxy, ArrayProxy, createListeners);
export const resolve = firstResolverFactory(Promise);

export const source = function(source) {
  // create API needed for conventional promise blocked, loading, Routes
  // i.e. resolve/reject on first response
  return resolve(source, createListeners()).then(function(data) {
    // create API needed for conventional DD/computed and Controllers
    return proxy(source, data);
  });
};
export const cache = cacheFactory(source, BlockingEventSource, Promise);

const errorEvent = function(e) {
  return new ErrorEvent('error', {
    error: e,
    message: e.message,
  });
};
export const fromPromise = function(promise) {
  return new CallableEventSource(function(configuration) {
    const dispatch = this.dispatchEvent.bind(this);
    const close = () => {
      this.close();
    };
    return promise
      .then(function(result) {
        close();
        dispatch({ type: 'message', data: result });
      })
      .catch(function(e) {
        close();
        dispatch(errorEvent(e));
      });
  });
};
export const toPromise = function(target, cb, eventName = 'message', errorName = 'error') {
  return new Promise(function(resolve, reject) {
    // TODO: e.target.data
    const message = function(e) {
      resolve(e.data);
    };
    const error = function(e) {
      reject(e.error);
    };
    const remove = function() {
      if (typeof target.close === 'function') {
        target.close();
      }
      target.removeEventListener(eventName, message);
      target.removeEventListener(errorName, error);
    };
    target.addEventListener(eventName, message);
    target.addEventListener(errorName, error);
    cb(remove);
  });
};
