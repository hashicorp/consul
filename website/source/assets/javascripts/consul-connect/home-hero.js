document.addEventListener('turbolinks:load', function () {
  var qsa = document.querySelectorAll.bind(document)
  var $$wrappers = qsa('#home-hero .videos > div') // all the wrappers for the videos
  var $$videos = qsa('#home-hero video') // all the videos
  var $$videoControls = qsa('#home-hero .controls > div') // carousel controllers
  var $$videoProgressBars =qsa('#home-hero .progress-bar span') // carousel controller progress bars
  var currentIndex = $$videos.length - 1 // currently playing video, inits as last video
  var playbackRate = 2 // video playback speed

  // if there are hero videos on the page, intiate the video carousel
  if ($$videos.length > 0) {
    setup()
  }

  // initiate the video carousel
  function setup() {
    // loop through videos to set up options/listeners
    for (var i = 0; i < $$wrappers.length; i++) {
      // some reseting that may need to be done
      // to work with turbolinks
      $$wrappers[i].classList.remove('active')
      $$videoProgressBars[i].style.width = 0
      $$videos[i].addEventListener('loadeddata', function () { 
        this.currentTime = 0 
      })

      // set video default speed
      $$videos[i].playbackRate = playbackRate

      // set up progress bar
      setupProgressBar($$videoProgressBars[i], $$videos[i])

      // listen for video ending, then go to the next video
      $$videos[i].addEventListener('ended', function() {
        var nextIndex = currentIndex + 1
        initiateVideoChange(nextIndex < $$videos.length ? nextIndex : 0)
      })

      // listen for control clicks and initiate videos appropriately
      $$videoControls[i].addEventListener('click', function() {
        if (!this.classList.contains('playing')) {
          initiateVideoChange(this.dataset.index)
        }
      })
    }

    // go to first video to start this thing
    initiateVideoChange(0)    
  }

  // set up progress bar
  function setupProgressBar(bar, video) {
    setInterval(function () {
      if (!isNaN(video.duration)) {
        bar.style.width = `${(video.currentTime / video.duration) * 100}%`
      }
    }, 200)
  }

  // initiate a video change
  function initiateVideoChange(nextIndex) {
    var currentWrapper = $$wrappers[currentIndex]
    var nextWrapper = $$wrappers[nextIndex]

    // pause the current video
    $$videos[currentIndex].pause()

    // deactivate the current video
    currentWrapper.classList.remove('active')
    currentWrapper.classList.add('deactivate')
    $$videoControls[currentIndex].classList.remove('playing')
    $$videoControls[nextIndex].classList.add('playing')

    // after the current video animates out...
    setTimeout(function() {
      // stop deactivation
      currentWrapper.classList.remove('deactivate')
      $$videos[currentIndex].currentTime = 0

      // check if the next video is loaded
      // if not, listen for it to load
      if ($$videos[nextIndex].readyState === 4) {
        $$videos[nextIndex].currentTime = 0
        playVideo(nextIndex, nextWrapper)
      } else {
        $$videos[nextIndex].addEventListener(
          'canplaythrough',
          playVideo(nextIndex, nextWrapper)
        )
      }
    }, 1000)
  }

  // activate and play a video
  function playVideo(nextIndex, nextWrapper) {
    // activate the next video and start playing it
    nextWrapper.classList.add('active')
    $$videos[nextIndex].play()

    // set the currentIndex to be that of the current video's index
    currentIndex = nextIndex
  }
})
