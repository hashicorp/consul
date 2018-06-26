var qs = document.querySelector.bind(document)
var qsa = document.querySelectorAll.bind(document)

var $$wrappers = qsa('#home-hero .videos > div') // all the wrappers for the videos
var $$videos = qsa('#home-hero video') // all the videos
var $$videoControls = qsa('#home-hero .controls > div') // carousel controllers
var currentIndex = 1 // currently playing video
var playbackRate = 2 // video playback speed

// initiate a video change
function initiateVideoChange(index) {
  var wrapper = $$wrappers[currentIndex]
  var nextWrapper = $$wrappers[index]

  // pause the current video
  $$videos[currentIndex].pause()

  // deactivate the current video
  wrapper.classList.remove('active')
  wrapper.classList.add('deactivate')

  // after the current video animates out...
  setTimeout(function() {
    // remove transition effect so progress-bar doesn't slowly decline
    var loadingBar = $$videoControls[currentIndex].querySelector(
      '.progress-bar span'
    )
    loadingBar.style.transitionDuration = '0s'

    // reset the current video
    if (!isNaN($$videos[currentIndex].duration)) {
      $$videos[currentIndex].currentTime = 0
    }
    $$videoControls[currentIndex].classList.remove('playing')

    // stop deactivation
    wrapper.classList.remove('deactivate')

    // check if the video is loaded
    // if not, listen for it to load
    if ($$videos[index].classList.contains('loaded')) {
      playVideo(index, nextWrapper)
    } else {
      $$videos[index].addEventListener(
        'canplaythrough',
        playVideo(index, nextWrapper)
      )
    }
  }, 1000)
}

// activate and play a video
function playVideo(index, wrapper) {
  // toggle
  $$videos[index].classList.add('loaded')

  // activate the next video and start playing it
  wrapper.classList.add('active')
  $$videoControls[index].classList.add('playing')
  $$videos[index].play()

  $$videoControls[index].querySelector(
    '.progress-bar span'
  ).style.transitionDuration =
    Math.ceil($$videos[index].duration / playbackRate).toString() + 's'

  // set the currentIndex to be that of the current video's index
  currentIndex = index
}

function initiateVideos() {
  // remove 'active' from wrappers which may be 
  // there on page load because of turbolinks
  for (var i = 0; i < $$wrappers.length; i++) {
    $$wrappers[i].classList.remove('active')
  }

  // loop through videos to set up options/listeners
  for (var i = 0; i < $$videos.length; i++) {
    // set video default speed
    $$videos[i].playbackRate = playbackRate

    // listen for video ending, then go to the next video
    $$videos[i].addEventListener('ended', function() {
      var nextIndex = currentIndex + 1
      initiateVideoChange(nextIndex < $$videos.length ? nextIndex : 0)
    })
  }

  for (var i = 0; i < $$videoControls.length; i++) {
    // remove 'playing' from controls which may be 
    // there on page load because of turbolinks
    $$videoControls[i].classList.remove('playing')

    // listen for control clicks and initiate videos appropriately
    $$videoControls[i].addEventListener('click', function() {
      if (!this.classList.contains('playing')) {
        initiateVideoChange(this.dataset.index)
      }
    })
  }

  // go to first video to start this thing
  if ($$videos.length > 0) {
    initiateVideoChange(0)
  }
}

document.addEventListener('turbolinks:load', initiateVideos)