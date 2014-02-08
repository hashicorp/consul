//
// util.js
//
var Serf = Serf || {};

(function () {

	//check for mobile user agents
	var isMobile = (function(){
		 if( navigator.userAgent.match(/Android/i)
		 || navigator.userAgent.match(/webOS/i)
		 || navigator.userAgent.match(/iPhone/i)
		 //|| navigator.userAgent.match(/iPad/i)
		 || navigator.userAgent.match(/iPod/i)
		 || navigator.userAgent.match(/BlackBerry/i)
		 || navigator.userAgent.match(/Windows Phone/i)
		 ){
			return true;
		  }
		 else {
		    return false;
		  }
    })()

    // calls the given function if the given classname is found
    function runIfClassNamePresent(selector, initFunction) {
        var elms = document.getElementsByClassName(selector);
        if (elms.length > 0) {
            initFunction();
        }
    }

    Serf.Util = {};
    Serf.Util.isMobile = isMobile;
    Serf.Util.runIfClassNamePresent = runIfClassNamePresent;

})();