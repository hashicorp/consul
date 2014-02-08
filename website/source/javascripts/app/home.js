//
// home.js
//
var Serf = Serf || {};

(function () {

    // calls the given function if the given classname is found
    function mobileHero() {
    	var jumbo = document.getElementById('jumbotron');
    	jumbo.className = jumbo.className + ' mobile-hero';
    }

    Serf.Home = {};
    Serf.Home.mobileHero = mobileHero;

})();