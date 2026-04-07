/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { skip, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { settled } from '@ember/test-helpers';
import Yadda from 'yadda';

import { env } from '../../env';
import api from './api';
import utils from './page';
import dictionary from '../dictionary';
const getDictionary = dictionary(utils);

const staticClassList = [...document.documentElement.classList];
const reset = function (owner) {
  // Wipe ALL cookies to prevent mutated permission cookies from leaking
  document.cookie.split(';').forEach((c) => {
    const name = c.split('=')[0].trim();
    if (name) {
      document.cookie = `${name}=; expires=${new Date(0).toUTCString()}`;
    }
  });
  window.localStorage.clear();
  api.server.reset();

  // Abort all in-flight connections (blocking queries)
  try {
    const connections = owner.lookup('service:client/connections');
    if (connections) {
      connections.purge();
    }
  } catch (e) {
    // service may already be destroyed
  }

  const list = document.documentElement.classList;
  while (list.length > 0) {
    list.remove(list.item(0));
  }
  staticClassList.forEach(function (item) {
    list.add(item);
  });
};
const startup = function () {
  api.server.setCookie('CONSUL_LATENCY', 0);
};

const runTest = function (context, libraries, steps, scenarioContext) {
  return new Promise((resolve, reject) => {
    Yadda.Yadda(libraries, context).yadda(steps, scenarioContext, function next(err, result) {
      if (err) {
        reject(err);
      }
      resolve(result);
    });
  });
};
const checkAnnotations = function (annotations, isScenario) {
  annotations = {
    namespaceable: env('CONSUL_NSPACES_ENABLED'),
    ...annotations,
  };
  if (annotations.ignore) {
    return function (test) {
      skip(`${test.title}`, function (assert) {});
    };
  }
  if (isScenario) {
    if (env('CONSUL_NSPACES_ENABLED')) {
      if (!annotations.notnamespaceable) {
        return function (scenario, feature, yadda, yaddaAnnotations, library) {
          const stepDefinitions = library.default;
          ['', 'default', 'team-1', undefined].forEach(function (item) {
            test(`Scenario: ${
              scenario.title
            } with the ${item === '' ? 'empty' : typeof item === 'undefined' ? 'undefined' : item} namespace set`, function (assert) {
              const scenarioContext = {
                ctx: {
                  nspace: item,
                },
              };
              const libraries = stepDefinitions({
                assert: assert,
                utils: utils,
                library: Yadda.localisation.English.library(getDictionary(annotations, item)),
              });
              return runTest(this, libraries, scenario.steps, scenarioContext);
            });
          });
        };
      } else {
        return function () {};
      }
    } else {
      if (!annotations.onlynamespaceable) {
        return function (scenario, feature, yadda, yaddaAnnotations, library) {
          const stepDefinitions = library.default;
          test(`Scenario: ${scenario.title}`, function (assert) {
            const scenarioContext = {
              ctx: {},
            };
            const libraries = stepDefinitions({
              assert: assert,
              utils: utils,
              library: Yadda.localisation.English.library(getDictionary(annotations)),
            });
            return runTest(this, libraries, scenario.steps, scenarioContext);
          });
        };
      } else {
        return function () {};
      }
    }
  }
};
export const setupFeature = function (featureAnnotations) {
  return setupApplicationTest;
};
export const setupScenario = function (featureAnnotations, scenarioAnnotations) {
  return function (model) {
    model.beforeEach(function () {
      startup();
    });
    model.afterEach(async function () {
      reset(this.owner);
      await settled();
    });
  };
};
export const runFeature = function (annotations) {
  return checkAnnotations(annotations);
};

export const runScenario = function (featureAnnotations, scenarioAnnotations) {
  return checkAnnotations({ ...featureAnnotations, ...scenarioAnnotations }, true);
};
