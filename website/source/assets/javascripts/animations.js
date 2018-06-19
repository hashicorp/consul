var qs = document.querySelector.bind(document)
var qsa = document.querySelectorAll.bind(document)

//
// configuration challenge animation
//

var configChallengeTimeline = new TimelineLite({
  onComplete: function() {
    $configChallenge.classList.remove('active')
    $configSolution.classList.add('active')
    configSolutionTimeline.restart()
  }
})

var line1 = qs('#c-line-1')
var line2 = qs('#c-line-2')
var line3 = qs('#c-line-3')
var line4 = qs('#c-line-4')
var line5 = qs('#c-line-5')
var line6 = qs('#c-line-6')
var line7 = qs('#c-line-7')
var line8 = qs('#c-line-8')
var box1 = qs('#c-box-1')
var box2 = qs('#c-box-2')
var box3 = qs('#c-box-3')
var box4 = qs('#c-box-4')
var box5 = qs('#c-box-5')
var box6 = qs('#c-box-6')
var box7 = qs('#c-box-7')
var box8 = qs('#c-box-8')
var progressBar = qs('#c-loading-bar > rect:last-child')
var cog = qs('#c-configuration-server > g > path')

configChallengeTimeline
  .to(box1, 1, {})
  .staggerTo(
    [line1, line2, line3, line4, line5, line6, line7, line8],
    1.5,
    { css: { strokeDashoffset: 0 } },
    0.3,
    'start'
  )
  .staggerTo(
    [box1, box2, box3, box4, box5, box6, box7, box8],
    0.3,
    { opacity: 1 },
    0.3,
    '-=2.5'
  )
  .to(progressBar, 3.5, { width: 40 }, 'start')
  .to(
    cog,
    3.5,
    { rotation: 360, svgOrigin: '136px 127px', ease: Power1.easeOut },
    'start'
  )
  .to(line1, 2, {})
  .to(
    [line1, line2, line3, line4, line5, line6, line7, line8, progressBar],
    0.5,
    { opacity: 0 },
    'reset'
  )
  .to(
    [box1, box2, box3, box4, box5, box6, box7, box8],
    0.5,
    { opacity: 0.5 },
    'reset'
  )
  .pause()

//
// configuration solution animation
//

var configSolutionTimeline = new TimelineLite({
  onComplete: function() {
    $configSolution.classList.remove('active')
    $configChallenge.classList.add('active')
    configChallengeTimeline.restart()
  }
})

var lines = qsa(
  '#s-line-1, #s-line-2, #s-line-3, #s-line-4, #s-line-5, #s-line-6, #s-line-7, #s-line-8'
)
var dots = qs('#s-dots')
var boxes = qsa(
  '#s-service-box-1, #s-service-box-2, #s-service-box-3, #s-service-box-4, #s-service-box-5, #s-service-box-6, #s-service-box-7, #s-service-box-8'
)
var progress = qs('#s-progress-indicator')

configSolutionTimeline
  .to(boxes, 1, {})
  .to(lines, 1, { css: { strokeDashoffset: 0 } }, 'start')
  .to(boxes, 0.5, { opacity: 1 }, '-=0.4')
  .to(progress, 1, { width: 40 }, 'start')
  .to(dots, 0.25, { opacity: 1 }, '-=0.5')
  .to(progress, 2, {})
  .to(lines, 0.5, { opacity: 0 }, 'reset')
  .to(boxes, 0.5, { opacity: 0.5 }, 'reset')
  .to(progress, 0.5, { opacity: 0 }, 'reset')
  .to(dots, 0.5, { opacity: 0 }, 'reset')
  .pause()

//
// configuration page
//

var $configChallenge = qs('#configuration-challenge-animation')
var $configSolution = qs('#configuration-solution-animation')

$configChallenge.classList.add('active')
configChallengeTimeline.play()
