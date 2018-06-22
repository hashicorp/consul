//
//
// home video carousel
var qs = document.querySelector.bind(document)
var qsa = document.querySelectorAll.bind(document)

var $$wrappers = qsa('#home-hero .videos > div') // all the wrappers for the videos
var $$videos = qsa('#home-hero video') // all the videos
var $$videoControls = qsa('#home-hero .controls > div') // carousel controllers
var currentIndex = 0 // currently playing video
var playbackRate = 2 // video playback speed

// initiate a video change
function initialiateVideoChange(index) {
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

    // reset the current video and stop deactivation
    $$videos[currentIndex].currentTime = 0
    $$videoControls[currentIndex].classList.remove('playing')

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
  ).style.transitionDuration = `${Math.ceil($$videos[index].duration / playbackRate)}s`

  // set the currentIndex to be that of the current video's index
  currentIndex = index
}

// loop through videos to set up options/listeners
for (var i = 0; i < $$videos.length; i++) {
  // set video default speed
  $$videos[i].playbackRate = playbackRate

  // listen for video ending, then go to the next video
  $$videos[i].addEventListener('ended', function() {
    var nextIndex = currentIndex + 1
    initialiateVideoChange(nextIndex < $$videos.length ? nextIndex : 0)
  })
}

for (var i = 0; i < $$videoControls.length; i++) {
  $$videoControls[i].addEventListener('click', function() {
    if (!this.classList.contains('playing')) {
      initialiateVideoChange(this.dataset.index)
    }
  })
}

// go to first video to start this thing
if ($$videos.length > 0) {
  initialiateVideoChange(0)
}

//
//
// siema carousels
var dots = qsa('.g-carousel .pagination li')
var carousel = new Siema({
  selector: '.siema',
  duration: 200,
  easing: 'ease-out',
  perPage: 1,
  startIndex: 0,
  draggable: true,
  multipleDrag: true,
  threshold: 20,
  loop: true,
  rtl: false,
  onChange: () => {
    for (var i = 0; i < dots.length; i++) {
      dots[i].classList.remove('active')
    }
    dots[carousel.currentSlide].classList.add('active')
  }
})

// on previous button click
document
  .querySelector('.g-carousel .prev')
  .addEventListener('click', function() {
    carousel.prev()
  })

// on next button click
document
  .querySelector('.g-carousel .next')
  .addEventListener('click', function() {
    carousel.next()
  })

// on dot click
for (let i = 0; i < dots.length; i++) {
  dots[i].addEventListener('click', function() {
    carousel.goTo(i)
  })
}
