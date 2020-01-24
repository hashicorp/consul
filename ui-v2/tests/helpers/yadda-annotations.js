import { skip, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { Promise } from 'rsvp';
import Yadda from 'yadda';

import { config } from '../../env';
import api from './api';
import getDictionary from '../dictionary';

const staticClassList = [...document.documentElement.classList];
const reset = function() {
  window.localStorage.clear();
  api.server.reset();
  const list = document.documentElement.classList;
  while (list.length > 0) {
    list.remove(list.item(0));
  }
  staticClassList.forEach(function(item) {
    list.add(item);
  });
};

const runTest = function(context, libraries, steps, scenarioContext) {
  return new Promise((resolve, reject) => {
    Yadda.Yadda(libraries, context).yadda(steps, scenarioContext, function next(err, result) {
      if (err) {
        reject(err);
      }
      resolve(result);
    });
  });
};
const checkAnnotations = function(annotations, isScenario) {
  annotations = {
    namespaceable: config('CONSUL_NSPACES_TEST'),
    ...annotations,
  };
  if (annotations.ignore) {
    return function(test) {
      skip(`${test.title}`, function(assert) {});
    };
  }
  if (isScenario) {
    return function(scenario, feature, yadda, yaddaAnnotations, library) {
      test(`Scenario: ${scenario.title}`, function(assert) {
        const libraries = library.default({
          assert: assert,
          library: Yadda.localisation.English.library(getDictionary()),
        });
        const scenarioContext = {
          ctx: {},
        };
        return runTest(this, libraries, scenario.steps, scenarioContext);
      });
      if (annotations.namespaceable && !annotations.notnamespaceable) {
        ['', 'default', 'team-1', undefined].forEach(function(item) {
          test(`Scenario: ${scenario.title} with the ${item} namespace set`, function(assert) {
            const libraries = library.default({
              assert: assert,
              library: Yadda.localisation.English.library(getDictionary(item)),
            });
            const scenarioContext = {
              ctx: {
                nspace: item,
              },
            };
            return runTest(this, libraries, scenario.steps, scenarioContext);
          });
        });
      }
    };
  }
};
export const setupFeature = function(featureAnnotations) {
  return setupApplicationTest;
};
export const setupScenario = function(featureAnnotations, scenarioAnnotations) {
  return function(model) {
    model.afterEach(function() {
      reset();
    });
  };
};
export const runFeature = function(annotations) {
  return checkAnnotations(annotations);
};

export const runScenario = function(featureAnnotations, scenarioAnnotations) {
  return checkAnnotations({ ...featureAnnotations, ...scenarioAnnotations }, true);
};
