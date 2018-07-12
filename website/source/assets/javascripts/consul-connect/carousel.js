objectFitImages()

var qs = document.querySelector.bind(document)
var qsa = document.querySelectorAll.bind(document)

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
  onChange: function() {
    for (var i = 0; i < dots.length; i++) {
      dots[i].classList.remove('active')
    }
    dots[carousel.currentSlide].classList.add('active')
  }
})

// on previous button click
qs('.g-carousel .prev')
  .addEventListener('click', function() {
    carousel.prev()
  })

// on next button click
qs('.g-carousel .next')
  .addEventListener('click', function() {
    carousel.next()
  })

// on dot click
for (var i = 0; i < dots.length; i++) {
  dots[i].addEventListener('click', function() {
    carousel.goTo(i)
  })
}
