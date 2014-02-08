//
// home.js
//

var Serf = (function() {

	function initialize (){
		Serf.Util.runIfClassNamePresent('page-home', initHome);
	}

	function initHome() {
		if(!Serf.Util.isMobile){
			Serf.Nodes.init(); 	
		}else{
			Serf.Home.mobileHero();
		}
		
	}
  
  	//api
	return {
		initialize: initialize
  	}

})();