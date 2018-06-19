var qs = document.querySelector.bind(document)
var qsa = document.querySelectorAll.bind(document)

//
// configuration page
//

var $configChallenge = qs('#configuration-challenge-animation')
var $configSolution = qs('#configuration-solution-animation')

if ($configChallenge) {
  // challenge animation

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

  // solution animation

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

  // kick off
  $configChallenge.classList.add('active')
  configChallengeTimeline.play()
}

//
// discovery page
//

var $discoveryChallenge = qs('#discovery-challenge-animation')
var $discoverySolution = qs('#discovery-solution-animation')

if ($discoveryChallenge) {
  // challenge animation
  var discoveryChallengeTimeline = new TimelineLite({
    onComplete: function() {
      $discoveryChallenge.classList.remove('active')
      $discoverySolution.classList.add('active')
      discoverySolutionTimeline.restart()
    }
  })

  // First, we get each of the elements we need to animate
  var box = qs('#c-active-box')
  var leftPlacement = qs('#c-box-left-placement')
  var rightPlacement = qs('#c-box-right-placement')
  var leftConnectionLines = qsa(
    '#c-line-top-left > *, #c-line-bottom-left > *, #c-line-horizontal-left > *, #c-line-vertical-down > *'
  )
  var rightConnectionLines = qsa(
    '#c-line-top-right > *, #c-line-bottom-right > *, #c-line-horizontal-left > *, #c-line-vertical-down > *, #c-line-horizontal-right > *'
  )
  var leftConnectionTop = qs('#c-line-top-left')
  var leftConnectionBottom = qs('#c-line-bottom-left')
  var rightHorizontalConnection = qs('#c-line-horizontal-right')
  var rightConnectionTop = qs('#c-line-top-right')
  var rightConnectionBottom = qs('#c-line-bottom-right')
  var rightConnectionLinesStroke = qsa(
    '#c-line-top-right > *, #c-line-bottom-right > *, #c-line-horizontal-right > *, #c-line-horizontal-left > *, #c-line-vertical-down > *'
  )
  var leftConnectionLinesStroke = qsa(
    '#c-line-top-left > *, #c-line-bottom-left > *, #c-line-horizontal-left > *, #c-line-vertical-down > *'
  )
  var brokenLinkLeft = qs('#c-broken-link-left')
  var brokenLinkRight = qs('#c-broken-link-right')
  var computer = qs('#c-computer')
  var codeLines = qs('#c-computer > g')
  var toLoadBalancerDown = qsa(
    '#c-computer-to-load-balancers #c-arrow-down, #c-computer-to-load-balancers #c-circle'
  )
  var toLoadBalancerRight = qs('#c-computer-to-load-balancers #c-arrow-right')
  var toLoadBalancerLeft = qs('#c-computer-to-load-balancers #c-arrow-left')
  var toLoadBalancerRest = qs('#c-computer-to-load-balancers #c-edit-box')
  var progressBars = qsa(
    '#c-load-balancer-left > #c-progress-bar, #c-load-balancer-right > #c-progress-bar-2, #c-load-balancer-middle > #c-progress-bar-3'
  )
  var progressBarsBars = qsa(
    '#c-load-balancer-left > #c-progress-bar > *:last-child, #c-load-balancer-right > #c-progress-bar-2  > *:last-child, #c-load-balancer-middle > #c-progress-bar-3  > *:last-child'
  )
  var farLeftBoxBorder = qs('#c-box-far-left > path')

  // Then, we run each step of the animation using GSAP's TimelineLine, a
  // fantastic way to set up a series of complex movements
  discoveryChallengeTimeline
    .to(box, 1, {})
    // box moves to new position
    .to(box, 1, { css: { transform: 'translate(96px, 48px)' } })
    .to(leftPlacement, 0.5, { css: { opacity: 1 } }, '-=1')
    .to(rightPlacement, 0.25, { css: { opacity: 0 } }, '-=0.25')
    // connection lines turn black
    .to(leftConnectionLines, 0.5, { css: { stroke: '#000' } })
    .to(farLeftBoxBorder, 0.5, { css: { fill: '#000' } }, '-=0.5')
    // broken link appears
    .to(
      leftConnectionTop,
      0.1,
      {
        css: { strokeDashoffset: 6 },
        ease: Linear.easeNone
      },
      '-=0.3'
    )
    .to(brokenLinkLeft, 0.2, { css: { opacity: 1 } }, '-=0.15')
    // computer appears and code is written
    .to(computer, 0.5, { css: { opacity: 1 } })
    .staggerFrom(
      codeLines,
      0.4,
      {
        css: { transform: 'translate(-64px, 0)', opacity: 0 }
      },
      0.1
    )
    .to(codeLines, 0.3, {
      css: { transform: 'translate(0, 0)', opacity: 1 }
    })
    // code moves to load balancers
    .to(toLoadBalancerRest, 0.4, { css: { opacity: 1 } })
    .to(toLoadBalancerLeft, 0.2, { css: { opacity: 1 } }, 'loadBalancerArrows')
    .to(toLoadBalancerRight, 0.2, { css: { opacity: 1 } }, 'loadBalancerArrows')
    .to(toLoadBalancerDown, 0.2, { css: { opacity: 1 } }, 'loadBalancerArrows')
    // load balancers progress bars, old broken link fades out
    .to(progressBars, 0.2, { css: { opacity: 1 } })
    .staggerTo(progressBarsBars, 1.5, { css: { width: 40 } }, 0.3)
    .to(
      []
        .concat([].slice.call(toLoadBalancerRest))
        .concat([].slice.call(toLoadBalancerDown))
        .concat([
          toLoadBalancerRight,
          toLoadBalancerLeft,
          computer,
          brokenLinkLeft,
          leftConnectionTop,
          leftConnectionBottom
        ]),
      0.5,
      { css: { opacity: 0 } },
      '-=0.75'
    )
    .to(progressBars, 0.5, { css: { opacity: 0 } })
    // new connection is drawn
    .to(rightHorizontalConnection, 0.3, { css: { strokeDashoffset: 0 } })
    .to(rightConnectionTop, 0.2, {
      css: { strokeDashoffset: 0 },
      ease: Linear.easeNone
    })
    .to(rightConnectionBottom, 0.3, {
      css: { strokeDashoffset: 0 },
      ease: Linear.easeNone
    })
    // connection lines turn blue
    .to(
      rightConnectionLinesStroke,
      0.5,
      { css: { stroke: '#3969ED' } },
      '-=0.3'
    )
    .to(farLeftBoxBorder, 0.5, { css: { fill: '#3969ED' } }, '-=0.5')
    // wait three seconds
    .to(box, 3, {})
    // box moves back to original position
    .to(box, 1, { css: { transform: 'translate(0, 0)' } }, 'loop2')
    .to(leftPlacement, 0.25, { css: { opacity: 0 } }, '-=0.25')
    .to(rightPlacement, 0.5, { css: { opacity: 1 } }, '-=0.5')
    // connection lines turn black
    .to(rightConnectionLines, 0.5, { css: { stroke: '#000' } })
    .to(farLeftBoxBorder, 0.5, { css: { fill: '#000' } }, '-=0.5')
    // broken link appears
    .to(
      rightConnectionTop,
      0.1,
      {
        css: { strokeDashoffset: 6 },
        ease: Linear.easeNone
      },
      '-=0.3'
    )
    .to(brokenLinkRight, 0.2, { css: { opacity: 1 } }, '-=0.15')
    // computer appears and code is written
    .from(codeLines, 0.1, { css: { opacity: 0 } })
    .to(computer, 0.5, { css: { opacity: 1 } }, '-=0.1')
    .staggerFromTo(
      codeLines,
      0.4,
      { css: { transform: 'translate(-64px, 0)', opacity: 0 } },
      { css: { transform: 'translate(0, 0)', opacity: 1 } },
      0.1
    )
    // code moves to load balancers
    .to(toLoadBalancerRest, 0.4, { css: { opacity: 1 } })
    .to(toLoadBalancerLeft, 0.2, { css: { opacity: 1 } }, 'loadBalancerArrows2')
    .to(
      toLoadBalancerRight,
      0.2,
      { css: { opacity: 1 } },
      'loadBalancerArrows2'
    )
    .to(toLoadBalancerDown, 0.2, { css: { opacity: 1 } }, 'loadBalancerArrows2')
    // load balancers progress bars, old broken link fades out
    .to(progressBarsBars, 0.1, { css: { width: 0 } })
    .to(progressBars, 0.2, { css: { opacity: 1 } })
    .staggerFromTo(
      progressBarsBars,
      1.5,
      { css: { width: 0 } },
      { css: { width: 40 } },
      0.3
    )
    .to(
      []
        .concat([].slice.call(toLoadBalancerRest))
        .concat([].slice.call(toLoadBalancerDown))
        .concat([
          toLoadBalancerRight,
          toLoadBalancerLeft,
          computer,
          brokenLinkRight,
          rightConnectionTop,
          rightConnectionBottom,
          rightHorizontalConnection
        ]),
      0.5,
      { css: { opacity: 0 } },
      '-=0.75'
    )
    .to(progressBars, 0.5, { css: { opacity: 0 } })
    // new connection is drawn
    .to(leftConnectionTop, 0.01, { css: { strokeDashoffset: 17 } })
    .to(leftConnectionBottom, 0.01, { css: { strokeDashoffset: 56 } })
    .to([leftConnectionTop, leftConnectionBottom], 0.01, {
      css: { opacity: 1 }
    })
    .to(leftConnectionTop, 0.2, {
      css: { strokeDashoffset: 0 },
      ease: Linear.easeNone
    })
    .to(leftConnectionBottom, 0.3, {
      css: { strokeDashoffset: 0 },
      ease: Linear.easeNone
    })
    // connection lines turn blue
    .to(leftConnectionLinesStroke, 0.5, { css: { stroke: '#3969ED' } }, '-=0.3')
    .to(farLeftBoxBorder, 0.5, { css: { fill: '#3969ED' } }, '-=0.5')
    .to(box, 2, {})
    .pause()

  // solution animation
  var discoverySolutionTimeline = new TimelineLite({
    onComplete: function() {
      $discoverySolution.classList.remove('active')
      $discoveryChallenge.classList.add('active')
      discoveryChallengeTimeline.restart()
    }
  })

  var inactiveBox = qs('#s-active-service-1')
  var inactiveBoxStroke = qs('#s-active-service-1 > path')
  var activeBox = qs('#s-active-service-2')
  var activeBoxStroke = qs('#s-active-service-2 > path')
  var leftPlacement = qs('#s-dotted-service-box-2')
  var rightPlacement = qs('#s-dotted-service-box-3')
  var leftConnectionLine = qs('#s-connected-line-1')
  var rightConnectionLine = qs('#s-connected-line-2')
  var dottedLineLeft = qs('#s-dotted-line-left')
  var dottedLineRight = qs('#s-dotted-lines-right')
  var dottedLineRightPrimary = qs('#s-dotted-lines-right > path:nth-child(2)')
  var dottedLineRightAlt = qs('#s-dotted-lines-right > path:last-child')
  var syncLeft = qs('#s-dynamic-sync-left')
  var syncRight = qs('#s-dynamic-sync-right')
  var syncSpinnerLeft = qs('#s-dynamic-sync-left > path')
  var syncSpinnerRight = qs('#s-dynamic-sync-right > path')

  discoverySolutionTimeline
    .to(activeBox, 1, {})
    // box moves
    .to(activeBox, 0.5, { x: 96, y: 48 })
    .to(leftPlacement, 0.25, { css: { opacity: 1 } }, '-=0.5')
    .to(rightPlacement, 0.25, { css: { opacity: 0 } }, '-=0.1')
    // connection is broken
    .to(leftConnectionLine, 0.75, { css: { strokeDashoffset: 222 } }, '-=0.5')
    // box color changes to black
    .to(activeBoxStroke, 0.25, { css: { fill: '#000' } }, '-=0.4')
    .to(inactiveBoxStroke, 0.25, { css: { fill: '#000' } }, '-=0.4')
    // right sync lines appear
    .to(dottedLineRight, 0.4, { css: { opacity: 1 } })
    .to(syncRight, 0.2, { css: { opacity: 1 } }, '-=0.2')
    .to(syncSpinnerRight, 1, { rotation: 360, svgOrigin: '232px 127px' })
    // left sync lines appear
    .to(dottedLineLeft, 0.4, { css: { opacity: 1 } }, '-=0.6')
    .to(syncLeft, 0.2, { css: { opacity: 1 } }, '-=0.2')
    .to(syncSpinnerLeft, 1, { rotation: 360, svgOrigin: '88px 127px' })
    // connection is redrawn
    .to(rightConnectionLine, 0.75, { css: { strokeDashoffset: 0 } })
    // right sync lines disappear
    .to(dottedLineRight, 0.4, { css: { opacity: 0 } }, '-=1.2')
    .to(syncRight, 0.2, { css: { opacity: 0 } }, '-=1.2')
    // left sync lines disappear
    .to(dottedLineLeft, 0.4, { css: { opacity: 0 } }, '-=0.5')
    .to(syncLeft, 0.2, { css: { opacity: 0 } }, '-=0.5')
    // box color changes to pink
    .to(activeBoxStroke, 0.25, { css: { fill: '#ca2171' } }, '-=0.2')
    .to(inactiveBoxStroke, 0.25, { css: { fill: '#ca2171' } }, '-=0.2')
    // wait three seconds
    .to(activeBox, 3, {})
    // box moves
    .to(activeBox, 0.5, { x: 0, y: 0 })
    .to(leftPlacement, 0.25, { css: { opacity: 0 } }, '-=0.1')
    .to(rightPlacement, 0.25, { css: { opacity: 1 } }, '-=0.5')
    // connection is broken
    .to(rightConnectionLine, 0.75, { css: { strokeDashoffset: 270 } }, '-=0.5')
    // box color changes to black
    .to(activeBoxStroke, 0.25, { css: { fill: '#000' } }, '-=0.4')
    .to(inactiveBoxStroke, 0.25, { css: { fill: '#000' } }, '-=0.4')
    // right sync lines appear
    .to(dottedLineRightAlt, 0.01, { css: { opacity: 1 } })
    .to(dottedLineRightPrimary, 0.01, { css: { opacity: 0 } })
    .to(dottedLineRight, 0.4, { css: { opacity: 1 } })
    .to(syncRight, 0.2, { css: { opacity: 1 } }, '-=0.2')
    .fromTo(
      syncSpinnerRight,
      1,
      { rotation: 0 },
      { rotation: 360, svgOrigin: '232px 127px' }
    )
    // left sync lines appear
    .to(dottedLineLeft, 0.4, { css: { opacity: 1 } }, '-=0.6')
    .to(syncLeft, 0.2, { css: { opacity: 1 } }, '-=0.2')
    .fromTo(
      syncSpinnerLeft,
      1,
      { rotation: 0 },
      { rotation: 360, svgOrigin: '88px 127px' }
    )
    // connection is redrawn
    .to(leftConnectionLine, 0.75, { css: { strokeDashoffset: 0 } })
    // right sync lines disappear
    .to(dottedLineRight, 0.4, { css: { opacity: 0 } }, '-=1.2')
    .to(syncRight, 0.2, { css: { opacity: 0 } }, '-=1.2')
    // left sync lines disappear
    .to(dottedLineLeft, 0.4, { css: { opacity: 0 } }, '-=0.5')
    .to(syncLeft, 0.2, { css: { opacity: 0 } }, '-=0.5')
    // box color changes to pink
    .to(activeBoxStroke, 0.25, { css: { fill: '#ca2171' } }, '-=0.2')
    .to(inactiveBoxStroke, 0.25, { css: { fill: '#ca2171' } }, '-=0.2')
    // wait three seconds
    .to(activeBox, 2, {})
    .pause()

  // kick it off
  $discoveryChallenge.classList.add('active')
  discoveryChallengeTimeline.play()
}
