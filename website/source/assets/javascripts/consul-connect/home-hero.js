document.addEventListener("turbolinks:load", function() {
  var qs = document.querySelector.bind(document);
  var qsa = document.querySelectorAll.bind(document);
  var $hero = qs("#home-hero");

  if ($hero) {
    var $$wrappers = qsa("#home-hero .videos > div"); // all the wrappers for the videos
    var $$videos = qsa("#home-hero video"); // all the videos
    var $$videoBars = qsa("#home-hero .progress-bar span"); // progress bars
    var $$videoControls = qsa("#home-hero .controls > div"); // carousel controllers
    var currentIndex = 1; // currently playing video
    var playbackRate = 2; // video playback speed

    // initiate a video change
    function initiateVideoChange(index) {
      var wrapper = $$wrappers[currentIndex];
      var nextWrapper = $$wrappers[index];

      // pause the current video
      $$videos[currentIndex].pause();

      // deactivate the current video
      wrapper.classList.remove("active");
      wrapper.classList.add("deactivate");

      // after the current video animates out...
      setTimeout(function() {
        // reset the current video
        if (!isNaN($$videos[currentIndex].duration)) {
          $$videos[currentIndex].currentTime = 0;
        }
        $$videoControls[currentIndex].classList.remove("playing");

        // stop deactivation
        wrapper.classList.remove("deactivate");

        // check if the video is loaded
        // if not, listen for it to load
        if ($$videos[index].classList.contains("loaded")) {
          playVideo(index, nextWrapper);
        } else {
          $$videos[index].addEventListener(
            "canplaythrough",
            playVideo(index, nextWrapper)
          );
        }
      }, 1000);
    }

    // activate and play a video
    function playVideo(index, wrapper) {
      // toggle
      $$videos[index].classList.add("loaded");

      // activate the next video and start playing it
      wrapper.classList.add("active");
      $$videoControls[index].classList.add("playing");
      $$videos[index].play();

      // sync playback bars to video.currentTime
      setInterval(() => {
        if (!isNaN($$videos[index].duration)) {
          $$videoBars[index].style.width = `${($$videos[index].currentTime /
            $$videos[index].duration) *
            100}%`;
        }
      }, 200);

      // set the currentIndex to be that of the current video's index
      currentIndex = index;
    }

    function initiateVideos() {
      // loop through videos to set up options/listeners
      for (var i = 0; i < $$videos.length; i++) {
        // set video default speed
        $$videos[i].playbackRate = playbackRate;

        // listen for video ending, then go to the next video
        $$videos[i].addEventListener("ended", function() {
          var nextIndex = currentIndex + 1;
          initiateVideoChange(nextIndex < $$videos.length ? nextIndex : 0);
        });
      }

      for (var i = 0; i < $$videoControls.length; i++) {
        // listen for control clicks and initiate videos appropriately
        $$videoControls[i].addEventListener("click", function() {
          if (!this.classList.contains("playing")) {
            initiateVideoChange(this.dataset.index);
          }
        });
      }

      // go to first video to start this thing
      if ($$videos.length > 0) {
        initiateVideoChange(0);
      }
    }

    initiateVideos();

    // reset it all
    document.addEventListener("turbolinks:before-cache", function() {
      for (var i = 0; i < $$videos.length; i++) {
        $$videos[i].currentTime = 0;
        $$videoBars[i].style.width = 0;
        $$videoControls[i].classList.remove("playing");
        $$wrappers[i].classList.remove("active");
      }
    });
  }
});
