//homepage.js

var APP = APP || {};

(function () {
  APP.Homepage = (function () {
    return {

      ui : null,

      init: function () {
        var _this = this;

        //cache elements
        this.ui = {
          $doc: $(window),
          $hero: $('#jumbotron')
        }
        
        this.addEventListeners();

      },

      addEventListeners: function(){
        var _this = this;

        if(APP.Utils.isMobile)
          return;
        
        _this.ui.$doc.scroll(function() {
          var top = _this.ui.$doc.scrollTop(),
              speedAdj = (top*0.8),
              speedAdjOffset = speedAdj - top;

          _this.ui.$hero.css('webkitTransform', 'translate(0, '+ speedAdj +'px)');
          _this.ui.$hero.find('.container').css('webkitTransform', 'translate(0, '+  speedAdjOffset +'px)');
        })
      }
    }
  }());

}(jQuery, this));

