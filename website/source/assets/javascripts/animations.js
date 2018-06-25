document.addEventListener('turbolinks:load', initializeAnimations)

function initializeAnimations() {
  var qs = document.querySelector.bind(document)
  var qsa = document.querySelectorAll.bind(document)

  //
  // home page
  //

  var $indexDynamic = qs('#index-dynamic-animation')
  if ($indexDynamic) {
    var initiated = false
    var observer = new IntersectionObserver(
      function(entries) {
        if (!initiated && entries[0].isIntersecting) {
          $indexDynamic.classList.add('active')
          var lines = qsa(
            '#lines-origin-aws > *, #lines-origin-azure > *, #lines-origin-gcp > *'
          )
          setTimeout(function() {
            timer = setInterval(function() {
              lines[parseInt(Math.random() * lines.length)].classList.toggle(
                'off'
              )
            }, 800)
          }, 3000)
          initiated = true
        }
      },
      { threshold: 0.5 }
    )
    observer.observe($indexDynamic)
  }

  //
  // configuration page
  //

  var $configChallenge = qs('#configuration-challenge-animation')
  var $configSolution = qs('#configuration-solution-animation')

  if ($configChallenge) {
    // challenge animation

    var configChallengeTimeline = new TimelineLite({
      onComplete: function() {
        configChallengeTimeline.restart()
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
      .fromTo(
        progressBar,
        3.5,
        { attr: { width: 0 } },
        { attr: { width: 40 } },
        'start'
      )
      .to(
        cog,
        3.5,
        { rotation: 360, svgOrigin: '136px 127px', ease: Power1.easeOut },
        'start'
      )
      .call(function () {
        configSolutionTimeline.resume(configSolutionTimeline.time())
      })
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

    var configSolutionTimeline = new TimelineLite()

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
      .fromTo(
        progress,
        1,
        { attr: { width: 0 } },
        { attr: { width: 40 } },
        'start'
      )
      .to(dots, 0.25, { opacity: 1 }, '-=0.5')
      .addPause()
      .to(progress, 2, {})
      .to(lines, 0.5, { opacity: 0 }, 'reset')
      .to(boxes, 0.5, { opacity: 0.5 }, 'reset')
      .to(progress, 0.5, { opacity: 0 }, 'reset')
      .to(dots, 0.5, { opacity: 0 }, 'reset')
      .pause()

    // kick off
    $configChallenge.classList.add('active')
    $configSolution.classList.add('active')
    configChallengeTimeline.play()
    configSolutionTimeline.play()
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
        discoveryChallengeTimeline.restart()
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
      .staggerFromTo(
        progressBarsBars,
        1.5,
        { attr: { width: 0 } },
        { attr: { width: 40 } },
        0.3
      )
      .to(
        []
          .concat(toLoadBalancerRest)
          .concat([].slice.call(toLoadBalancerDown))
          .concat([
            toLoadBalancerRight,
            toLoadBalancerLeft,
            brokenLinkLeft,
            leftConnectionTop,
            leftConnectionBottom
          ]),
        0.5,
        { css: { opacity: 0 } },
        '-=0.75'
      )
      .to(computer, 0.5, { css: { opacity: .12 } }, '-=0.75')
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
      .to(progressBarsBars, 0.1, { attr: { width: 0 } })
      .to(progressBars, 0.2, { attr: { opacity: 1 } })
      .staggerFromTo(
        progressBarsBars,
        1.5,
        { css: { width: 0 } },
        { css: { width: 40 } },
        0.3
      )
      .to(
        []
          .concat(toLoadBalancerRest)
          .concat([].slice.call(toLoadBalancerDown))
          .concat([
            toLoadBalancerRight,
            toLoadBalancerLeft,
            brokenLinkRight,
            rightConnectionTop,
            rightConnectionBottom,
            rightHorizontalConnection
          ]),
        0.5,
        { css: { opacity: 0 } },
        '-=0.75'
      )
      .to(computer, 0.5, { css: { opacity: .12 } }, '-=0.75')
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
      .call(function () {
        discoverySolutionTimeline.resume(discoverySolutionTimeline.time())
      })
      .to(box, 2, {})
      .pause()

    // solution animation
    var discoverySolutionTimeline = new TimelineLite()

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
      .addPause()
      // wait three seconds
      .to(activeBox, 2, {})
      .pause()

    // kick it off
    $discoveryChallenge.classList.add('active')
    $discoverySolution.classList.add('active')
    discoveryChallengeTimeline.play()
    discoverySolutionTimeline.play()
  }

  //
  // discovery page
  //

  var $segmentationChallenge = qs('#segmentation-challenge-animation')
  var $segmentationSolution = qs('#segmentation-solution-animation')

  if ($segmentationChallenge) {
    // challenge animation
    var segmentationChallengeTimeline = new TimelineLite({
      onComplete: function() {
        segmentationChallengeTimeline.restart()
        segmentationSolutionTimeline.restart()
      }
    })

    var computerUpdatePath = qs('#c-firewall-updates #c-update_path')
    var computerUpdateBox = qs('#c-firewall-updates #c-edit')
    var computer = qs('#c-computer')
    var progressBars = qsa(
      '#c-progress-indicator, #c-progress-indicator-2, #c-progress-indicator-3'
    )
    var progressBarBars = qsa(
      '#c-progress-indicator > rect:last-child, #c-progress-indicator-2 > rect:last-child, #c-progress-indicator-3 > rect:last-child'
    )
    var brokenLinks = qsa('#c-broken-link-1, #c-broken-link-2, #c-broken-link-3')
    var box2 = qs('#c-box-2')
    var box2Border = qs('#c-box-2 > path')
    var box4 = qs('#c-box-4')
    var box4Border = qs('#c-box-4 > path')
    var box6 = qs('#c-box-6')
    var box6Border = qs('#c-box-6 > path')
    var box7 = qs('#c-box-7')
    var box7Border = qs('#c-box-7 > path')
    var path1a = qs('#c-path-1 > *:nth-child(2)')
    var path1b = qs('#c-path-1 > *:nth-child(3)')
    var path1c = qs('#c-path-1 > *:nth-child(1)')
    var path2a = qs('#c-path-2 > *:nth-child(1)')
    var path2b = qs('#c-path-2 > *:nth-child(3)')
    var path2c = qs('#c-path-2 > *:nth-child(2)')
    var path3a = qs('#c-path-3 > *:nth-child(2)')
    var path3b = qs('#c-path-3 > *:nth-child(3)')
    var path3c = qs('#c-path-3 > *:nth-child(1)')

    segmentationChallengeTimeline
      .to(box2, 1, {})
      // box 4 and 6 appear
      .to(box4Border, 0.4, { css: { fill: '#000' } }, 'box4-in')
      .fromTo(
        box4,
        0.3,
        { scale: 0, rotation: 200, opacity: 0, svgOrigin: '291px 41px' },
        { scale: 1, rotation: 360, opacity: 1 },
        'box4-in'
      )
      .to(box6Border, 0.4, { css: { fill: '#000' } }, '-=0.2')
      .fromTo(
        box6,
        0.3,
        { scale: 0, rotation: 200, opacity: 0, svgOrigin: '195px 289px' },
        { scale: 1, rotation: 360, opacity: 1 },
        '-=0.4'
      )
      // wait for a moment
      .to(box2, 1, {})
      // computer appears and sends updates to firewalls
      .to(computer, 0.5, { opacity: 1 })
      .to(computerUpdateBox, 0.3, { opacity: 1 }, '-=0.2')
      .to(computerUpdatePath, 0.3, { opacity: 1 }, '-=0.2')
      // firewall progress bars
      .to(progressBarBars, 0.01, { attr: { width: 0 } })
      .to(progressBars, 0.2, { opacity: 1 })
      .staggerTo(progressBarBars, 0.6, { attr: { width: 40 } }, 0.2)
      // connection 1 made
      .to(path1a, 0.3, { css: { strokeDashoffset: 0 }, ease: Linear.easeNone })
      .to(path1b, 0.3, { css: { strokeDashoffset: 0 }, ease: Linear.easeNone })
      .to(path1c, 0.3, { css: { strokeDashoffset: 0 }, ease: Linear.easeNone })
      // progress bars and firewall update lines fade out
      .to(progressBars, 0.7, { opacity: 0 }, 'resetComputer1')
      .to(computerUpdateBox, 0.7, { opacity: 0 }, 'resetComputer1')
      .to(computerUpdatePath, 0.7, { opacity: 0 }, 'resetComputer1')
      // connection turns blue
      .to(
        [path1a, path1b, path1c],
        0.5,
        { css: { stroke: '#3969ED' } },
        'resetComputer1'
      )
      .to(
        [box4Border, box6Border],
        0.5,
        { css: { fill: '#3969ED' } },
        'resetComputer1'
      )
      // second connection draws
      .to(
        path2a,
        0.3,
        { css: { strokeDashoffset: 0 }, ease: Linear.easeNone },
        '-=0.3'
      )
      .to(path2b, 0.3, { css: { strokeDashoffset: 0 }, ease: Linear.easeNone })
      .to(path2c, 0.2, { css: { strokeDashoffset: 0 }, ease: Linear.easeNone })
      // second connection turns blue
      .to([path2a, path2b, path2c], 0.5, { css: { stroke: '#3969ED' } }, '-=0.1')
      .to(box7Border, 0.5, { css: { fill: '#3969ED' } }, '-=0.3')
      // wait a moment
      .to(box2, 2, {})
      // blue elements fade back to gray
      .to(
        [path1a, path1b, path1c, path2a, path2b, path2c],
        0.5,
        {
          css: { stroke: '#b5b8c4' }
        },
        'colorReset1'
      )
      .to(
        [box7Border, box4Border, box6Border],
        0.5,
        { css: { fill: '#b5b8c4' } },
        'colorReset1'
      )
      // box 2 appears
      .to(box2Border, 0.4, { css: { fill: '#000' } }, 'colorReset1')
      .fromTo(
        box2,
        0.3,
        { scale: 0, rotation: 200, opacity: 0, svgOrigin: '195px 42px' },
        { scale: 1, rotation: 360, opacity: 1 },
        '-=0.4'
      )
      // wait a moment
      .to(box2, 1, {})
      // computer updates firewalls
      .to(computerUpdateBox, 0.3, { opacity: 1 }, '-=0.2')
      .to(computerUpdatePath, 0.3, { opacity: 1 }, '-=0.2')
      // firewall progress bars
      .to(progressBarBars, 0.01, { width: 0 })
      .to(progressBars, 0.2, { opacity: 1 })
      .staggerTo(progressBarBars, 0.6, { width: 40 }, 0.2)
      // third connection made
      .to(path3a, 0.3, { css: { strokeDashoffset: 0 }, ease: Linear.easeNone })
      .to(path3b, 0.3, { css: { strokeDashoffset: 0 }, ease: Linear.easeNone })
      .to(path3c, 0.3, { css: { strokeDashoffset: 0 }, ease: Linear.easeNone })
      // progress bars & computer arrows fade out
      .to(progressBars, 0.5, { opacity: 0 }, 'computerReset2')
      .to(computerUpdateBox, 0.5, { opacity: 0 }, 'computerReset2')
      .to(computerUpdatePath, 0.5, { opacity: 0 }, 'computerReset2')
      // third connection turns blue
      .to(
        [path3a, path3b, path3c],
        0.5,
        { css: { stroke: '#3969ED' } },
        'computerReset2'
      )
      .to(
        [box2Border, box7Border],
        0.5,
        { css: { fill: '#3969ED' } },
        'computerReset2'
      )
      // wait a bit
      .to(box2, 2, {})
      // third connection turns back to gray
      .to(
        [path3a, path3b, path3c],
        0.5,
        { css: { stroke: '#b5b8c4' } },
        'colorReset2'
      )
      .to(
        [box2Border, box7Border],
        0.5,
        { css: { fill: '#b5b8c4' } },
        'colorReset2'
      )
      // boxes 2, 4, and 6 disappear
      .to(
        [box2, box4, box6],
        0.6,
        { scale: 0, rotation: 200, opacity: 0 },
        '-=0.4'
      )
      // lines turn red and broken links appear
      .to(
        [path1a, path1b, path1c, path2a, path2b, path2c, path3a, path3b, path3c],
        0.3,
        { css: { stroke: '#ED4168' } },
        '-=0.2'
      )
      .to(brokenLinks, 0.3, { opacity: 1 }, '-=0.3')
      // wait a moment
      .to(box2, 1, {})
      // code sent to firewalls
      .to(computerUpdateBox, 0.3, { opacity: 1 })
      .to(computerUpdatePath, 0.3, { opacity: 1 })
      // firewall progress bars
      .to(progressBarBars, 0.01, { width: 0 })
      .to(progressBars, 0.2, { opacity: 1 })
      .staggerTo(progressBarBars, 0.6, { width: 40 }, 0.2)
      .to(box2, 0.5, {})
      // faulty connections removed
      .to(
        [
          path1a,
          path1b,
          path1c,
          path2a,
          path2b,
          path2c,
          path3a,
          path3b,
          path3c
        ].concat(brokenLinks),
        0.7,
        { opacity: 0 }
      )
      // progress bars and connection arrows fade out
      .to(progressBars, 0.5, { opacity: 0 }, 'computerReset3')
      .to(computerUpdateBox, 0.5, { opacity: 0 }, 'computerReset3')
      .to(computerUpdatePath, 0.5, { opacity: 0 }, 'computerReset3')
      .to(computer, 0.5, { opacity: 0 }, 'computerReset3')
      .call(function () {
        segmentationSolutionTimeline.resume(segmentationSolutionTimeline.time())
      })
      // wait a moment before the loop
      .to(box2, 1, {})
      .pause()

    // solution animation
    var segmentationSolutionTimeline = new TimelineLite()

    // service boxes
    var box1 = qs('#s-service-2')
    var box1Border = qs('#s-service-2 > path')
    var box1Lock = qs('#s-service-2 #s-secure-indicator-2')
    var box2 = qs('#s-service-4')
    var box2Border = qs('#s-service-4 > path')
    var box2Lock = qs('#s-service-4 #s-secure-indicator-4')
    var box3 = qs('#s-service-6')
    var box3Border = qs('#s-service-6 > path')
    var box3Lock = qs('#s-service-6 #s-secure-indicator-6')

    // connection paths
    var path1a = qs('#s-connection-path-2')
    var path1b = qs('#s-connection-path-8')
    var path2a = qs('#s-connection-path-9')
    var path2b = qs('#s-connection-path-10')
    var path3a = qs('#s-connection-path-1')
    var path3b = qs('#s-connection-path-4')
    var path3c = qs('#s-connection-path-5')
    var path3d = qs('#s-connection-path-6')

    // inbound consul updates
    var inboundPathLower = qs('#s-consul-inbound-paths-lower')
    var inboundUpdateLower = qs('#s-dynamic-update-inbound-lower')
    var inboundUpdateLowerSpinner = qs('#s-dynamic-update-inbound-lower > path')
    var inboundPathUpper = qs('#s-consul-inbound-paths-upper')
    var inboundUpdateUpper = qs('#s-dynamic-update-inbound-upper')
    var inboundUpdateUpperSpinner = qs('#s-dynamic-update-inbound-upper > path')

    // outbound consul updates
    var outboundPathsLower = qsa(
      '#s-consul-server-connection-lower, #s-consul-outbound-5, #s-consul-outbound-6, #s-consul-outbound-7'
    )
    var outboundUpdateLower = qsa(
      '#s-dynamic-update-outbound-ower, #s-tls-cert-lower'
    )
    var outboundUpdateLowerSpinner = qs('#s-dynamic-update-outbound-ower > path')
    var outboundPathsUpper1 = qsa(
      '#s-consul-server-connection-upper, #s-consul-outbound-3, #s-consul-outbound-4'
    )
    var outboundPathsUpper2 = qsa(
      '#s-consul-server-connection-upper, #s-consul-outbound-1, #s-soncul-outbound-2'
    )
    var outboundUpdateUpper = qsa(
      '#s-tls-cert-upper, #s-dynamic-update-outbound-upper'
    )
    var outboundUpdateUpperSpinner = qs('#s-dynamic-update-outbound-upper > path')

    segmentationSolutionTimeline
      .to(box2, 1, {})
      // boxes 2 and 3 appear
      .fromTo(
        box2,
        0.3,
        { scale: 0, rotation: 200, opacity: 0, svgOrigin: '281px 104px' },
        { scale: 1, rotation: 360, opacity: 1 }
      )
      .fromTo(
        box3,
        0.3,
        { scale: 0, rotation: 200, opacity: 0, svgOrigin: '185px 226px' },
        { scale: 1, rotation: 360, opacity: 1 },
        '-=0.1'
      )
      // wait a moment
      .to(box1, 0.5, {})
      // consul speaks to each box that needs a connection made
      .to(outboundPathsUpper1, 0.5, { opacity: 1 })
      .to(outboundPathsLower, 0.5, { opacity: 1 }, '-=0.3')
      .to(outboundUpdateUpper, 0.3, { opacity: 1 }, '-=0.3')
      .to(outboundUpdateLower, 0.3, { opacity: 1 }, '-=0.1')
      .to(
        outboundUpdateUpperSpinner,
        0.7,
        {
          rotation: 360,
          svgOrigin: '44px 99px'
        },
        '-=0.5'
      )
      .to(
        outboundUpdateLowerSpinner,
        0.7,
        {
          rotation: 360,
          svgOrigin: '44px 246px'
        },
        '-=0.3'
      )
      // pink borders, locks, connections drawn, consul talk fades
      .to(box2Lock, 0.3, { opacity: 1 }, 'connections-1')
      .to(box2Border, 0.3, { fill: '#CA2270' }, 'connections-1')
      .to(box3Lock, 0.3, { opacity: 1 }, 'connections-1')
      .to(box3Border, 0.3, { fill: '#CA2270' }, 'connections-1')
      .to(outboundPathsUpper1, 0.7, { opacity: 0 }, 'connections-1')
      .to(outboundPathsLower, 0.7, { opacity: 0 }, 'connections-1')
      .to(outboundUpdateUpper, 0.7, { opacity: 0 }, 'connections-1')
      .to(outboundUpdateLower, 0.7, { opacity: 0 }, 'connections-1')
      .to(
        path1a,
        0.5,
        { css: { strokeDashoffset: 0, stroke: '#CA2270' } },
        'connections-1'
      )
      .to(
        path1b,
        0.5,
        { css: { strokeDashoffset: 0, stroke: '#CA2270' } },
        'connections-1'
      )
      .to(
        path2a,
        0.5,
        { css: { strokeDashoffset: 0, stroke: '#CA2270' } },
        'connections-1'
      )
      .to(
        path2b,
        0.5,
        { css: { strokeDashoffset: 0, stroke: '#CA2270' } },
        'connections-1'
      )
      // wait a moment
      .to(box1, 0.5, {})
      // box 1 appears
      .fromTo(
        box1,
        0.3,
        { scale: 0, rotation: 200, opacity: 0, svgOrigin: '185px 104px' },
        { scale: 1, rotation: 360, opacity: 1 },
        '-=0.1'
      )
      // wait a moment, previous paths fade ('#EEB9D1')
      .to(box1, 0.5, {}, 'stage-1-complete')
      .to(box2Border, 0.5, { fill: '#EEB9D1' }, 'stage-1-complete')
      .to(box3Border, 0.5, { fill: '#EEB9D1' }, 'stage-1-complete')
      .to(path1a, 0.5, { css: { stroke: '#EEB9D1' } }, 'stage-1-complete')
      .to(path1b, 0.5, { css: { stroke: '#EEB9D1' } }, 'stage-1-complete')
      .to(path2a, 0.5, { css: { stroke: '#EEB9D1' } }, 'stage-1-complete')
      .to(path2b, 0.5, { css: { stroke: '#EEB9D1' } }, 'stage-1-complete')
      // consul speaks to each box that needs a connection made
      .to(outboundPathsUpper2, 0.5, { opacity: 1 })
      .to(outboundPathsLower, 0.5, { opacity: 1 }, '-=0.3')
      .to(outboundUpdateUpper, 0.3, { opacity: 1 }, '-=0.3')
      .to(outboundUpdateLower, 0.3, { opacity: 1 }, '-=0.1')
      .to(
        outboundUpdateUpperSpinner,
        0.7,
        {
          rotation: 720,
          svgOrigin: '44px 99px'
        },
        '-=0.5'
      )
      .to(
        outboundUpdateLowerSpinner,
        0.7,
        {
          rotation: 720,
          svgOrigin: '44px 246px'
        },
        '-=0.3'
      )
      // connections drawn
      .to(box1Lock, 0.3, { opacity: 1 }, 'connections-2')
      .to(box1Border, 0.3, { fill: '#CA2270' }, 'connections-2')
      .to(
        path3a,
        0.5,
        { css: { strokeDashoffset: 0, stroke: '#CA2270' } },
        'connections-2'
      )
      .to(
        path3b,
        0.5,
        { css: { strokeDashoffset: 0, stroke: '#CA2270' } },
        'connections-2'
      )
      .to(
        path3c,
        0.5,
        { css: { strokeDashoffset: 0, stroke: '#CA2270' } },
        'connections-2'
      )
      .to(
        path3d,
        0.5,
        { css: { strokeDashoffset: 0, stroke: '#CA2270' } },
        'connections-2'
      )
      .to(box1, 0.7, {}, 'stage-2-complete')
      .to(outboundPathsUpper2, 0.7, { opacity: 0 }, 'stage-2-complete')
      .to(outboundPathsLower, 0.7, { opacity: 0 }, 'stage-2-complete')
      .to(outboundUpdateUpper, 0.7, { opacity: 0 }, 'stage-2-complete')
      .to(outboundUpdateLower, 0.7, { opacity: 0 }, 'stage-2-complete')
      .to(box1Border, 0.5, { fill: '#EEB9D1' }, 'path-fade-2')
      .to(path3a, 0.5, { css: { stroke: '#EEB9D1' } }, 'path-fade-2')
      .to(path3b, 0.5, { css: { stroke: '#EEB9D1' } }, 'path-fade-2')
      .to(path3c, 0.5, { css: { stroke: '#EEB9D1' } }, 'path-fade-2')
      .to(path3d, 0.5, { css: { stroke: '#EEB9D1' } }, 'path-fade-2')
      // wait a moment
      .to(box1, 1, {})
      // all new boxes and connections fade
      .to(
        [
          box1,
          box2,
          box3,
          path1a,
          path1b,
          path2a,
          path2b,
          path3a,
          path3b,
          path3c,
          path3d
        ],
        0.5,
        { opacity: 0.3 }
      )
      // faded boxes speak to consul
      .to(inboundPathLower, 0.5, { opacity: 1 }, 'inbound')
      .to(inboundPathUpper, 0.5, { opacity: 1 }, 'inbound')
      .to(inboundUpdateLower, 0.5, { opacity: 1 }, 'inbound')
      .to(inboundUpdateUpper, 0.5, { opacity: 1 }, 'inbound')
      .to(
        inboundUpdateLowerSpinner,
        0.7,
        {
          rotation: 360,
          svgOrigin: '44px 237px'
        },
        '-=0.3'
      )
      .to(
        inboundUpdateUpperSpinner,
        0.7,
        {
          rotation: 360,
          svgOrigin: '44px 91px'
        },
        '-=0.3'
      )
      // consul removes faded boxes and connections
      .to(
        [
          box1,
          box2,
          box3,
          path1a,
          path1b,
          path2a,
          path2b,
          path3a,
          path3b,
          path3c,
          path3d,
          inboundPathLower,
          inboundPathUpper,
          inboundUpdateLower,
          inboundUpdateUpper
        ],
        0.5,
        { opacity: 0.0 }
      )
      .addPause()
      // wait a moment before the loop
      .to(box1, 1, {})
      .pause()

    // kick it off
    $segmentationChallenge.classList.add('active')
    $segmentationSolution.classList.add('active')
    segmentationChallengeTimeline.play()
    segmentationSolutionTimeline.play()
  }
}