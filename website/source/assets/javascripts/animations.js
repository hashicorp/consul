//
// configuration challenge animation
//

var qs = document.querySelector.bind(document)
var t = new TimelineLite({
  onComplete: function() {
    this.restart()
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

t
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
  .to(line1, 1, {})
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
  .to(line1, 2, {})
