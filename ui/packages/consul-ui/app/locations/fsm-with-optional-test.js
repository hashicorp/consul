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
      // getting rootURL straight from env would be nicer but is non-standard
      // and we still need access to router above
      this.rootURL = router.rootURL.replace(/\/$/, '');
      // do some pre-setup setup so getURL can work
      // this is machine setup that would be nice to via machine
      // instantiation, its basically initialState
      // move machine instantiation here once its an event target
      this.machine.state.path = this.location.pathname = `${this.rootURL}${path}`;
      this.path = this.getURL();
      // handleURL calls setupRouter for us
      return app.handleURL(`${this.path}`).then(handleTransitionResolve, handleTransitionReject);
    }
    // anything else, just transitionTo like normal
    return this.transitionTo(path).then(handleTransitionResolve, handleTransitionReject);
  }
}
