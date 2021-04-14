import FSMWithOptionalLocation from './fsm-with-optional';
import { FSM, Location } from './fsm';

import { settled } from '@ember/test-helpers';

export default class FSMWithOptionalTestLocation extends FSMWithOptionalLocation {
  implementation = 'fsm-with-optional-test';
  static create() {
    return new this(...arguments);
  }
  constructor() {
    super(...arguments);
    this.location = new Location();
    this.machine = new FSM(this.location);

    // Browsers add event listeners to the state machine via the
    // document/defaultView
    this.doc = {
      defaultView: {
        addEventListener: (event, cb) => {
          this.machine = new FSM(this.location, cb);
        },
        removeEventListener: (event, cb) => {
          this.machine = new FSM();
        },
      },
    };
  }

  visit(path) {
    const app = this.container;
    const router = this.container.lookup('router:main');

    // taken from emberjs/application/instance:visit but cleaned up a little
    // https://github.com/emberjs/ember.js/blob/21bd70c773dcc4bfe4883d7943e8a68d203b5bad/packages/%40ember/application/instance.js#L236-L277
    const handleTransitionResolve = async _ => {
      await settled();
      return new Promise(resolve => setTimeout(resolve(app), 0));
    };
    const handleTransitionReject = error => {
      if (error.error) {
        throw error.error;
      } else if (error.name === 'TransitionAborted' && router._routerMicrolib.activeTransition) {
        return router._routerMicrolib.activeTransition.then(
          handleTransitionResolve,
          handleTransitionReject
        );
      } else if (error.name === 'TransitionAborted') {
        throw new Error(error.message);
      } else {
        throw error;
      }
    };
    //

    // the first time around, set up location via handleURL
    if (this.location.pathname === '') {
      const url = this.getURLForTransition(path);
      // detect lets us set these properties in the correct order
      this.detect = function() {
        this.path = url;
        this.machine.state.path = this.location.pathname = `${this.rootURL.replace(
          /\/$/,
          ''
        )}${path}`;
      };
      // handleURL calls setupRouter for us
      return app.handleURL(`${url}`).then(handleTransitionResolve, handleTransitionReject);
    }
    // anything else, just transitionTo like normal
    return this.transitionTo(path).then(handleTransitionResolve, handleTransitionReject);
  }
}
