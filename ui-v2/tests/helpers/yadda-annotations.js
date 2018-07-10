import ENV from '../../config/environment';
import { skip } from 'qunit';
import { setupApplicationTest, setupRenderingTest, setupTest } from 'ember-qunit';
import api from 'consul-ui/tests/helpers/api';

const staticClassList = [...document.documentElement.classList];
function reset() {
  window.localStorage.clear();
  api.server.reset();
  const list = document.documentElement.classList;
  while (list.length > 0) {
    list.remove(list.item(0));
  }
  staticClassList.forEach(function(item) {
    list.add(item);
  });
}
// this logic could be anything, but in this case...
// if @ignore, then return skip (for backwards compatibility)
// if have annotations in config, then only run those that have a matching annotation
function checkAnnotations(annotations) {
  // if ignore is set then we want to skip for backwards compatibility
  if (annotations.ignore) {
    return ignoreIt;
  }

  // if have annotations set in config, the only run those that have a matching annotation
  if (ENV.annotations && ENV.annotations.length >= 0) {
    for (let annotation in annotations) {
      if (ENV.annotations.indexOf(annotation) >= 0) {
        // have match, so test it
        return 'testIt'; // return something other than a function
      }
    }

    // no match, so don't run it
    return logIt;
  }
}

// call back functions
function ignoreIt(testElement) {
  skip(`${testElement.title}`, function(/*assert*/) {});
}

function logIt(testElement) {
  console.info(`Not running or skipping: "${testElement.title}"`); // eslint-disable-line no-console
}

// exported functions
function runFeature(annotations) {
  return checkAnnotations(annotations);
}

function runScenario(featureAnnotations, scenarioAnnotations) {
  return checkAnnotations(scenarioAnnotations);
}

// setup tests
// you can override these function to add additional setup setups, or handle new setup related annotations
function setupFeature(featureAnnotations) {
  return setupYaddaTest(featureAnnotations);
}

function setupScenario(featureAnnotations, scenarioAnnotations) {
  let setupFn = setupYaddaTest(scenarioAnnotations);
  if (
    setupFn &&
    (featureAnnotations.setupapplicationtest ||
      featureAnnotations.setuprenderingtest ||
      featureAnnotations.setuptest)
  ) {
    throw new Error(
      'You must not assign any @setupapplicationtest, @setuprenderingtest or @setuptest annotations to a scenario as well as its feature!'
    );
  }
  return function(model) {
    model.afterEach(function() {
      reset();
    });
  };
  // return setupFn;
}

function setupYaddaTest(annotations) {
  if (annotations.setupapplicationtest) {
    return setupApplicationTest;
  }
  if (annotations.setuprenderingtest) {
    return setupRenderingTest;
  }
  if (annotations.setuptest) {
    return setupTest;
  }
}

export { runFeature, runScenario, setupFeature, setupScenario };
