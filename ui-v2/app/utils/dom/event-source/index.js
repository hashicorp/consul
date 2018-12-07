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
export const proxy = proxyFactory(ObjectProxy, ArrayProxy);
export const resolve = firstResolverFactory(Promise);

export const source = function(source) {
  // create API needed for conventional promise blocked, loading, Routes
  // i.e. resolve/reject on first response
  return resolve(source, createListeners()).then(function(data) {
    // create API needed for conventional DD/computed and Controllers
    return proxy(data, source, createListeners());
  });
};
export const cache = cacheFactory(source, BlockingEventSource, Promise);
