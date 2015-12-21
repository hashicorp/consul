//
// app.js
//

var APP = (function() {

  function initializeSidebar() {
    new Sidebar();
  }

  function initialize() {
    APP.Utils.runIfClassNamePresent('page-home', initHome);

    //always init sidebar
    initializeSidebar();
  }

  function initHome() {
    APP.Homepage.init();
  }

  //api
  return {
    initialize: initialize
  }

})();
