var dots = document.querySelectorAll('.g-carousel .pagination li')
if (document.querySelector('.siema')) {
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

  document
    .querySelector('.g-carousel .prev')
    .addEventListener('click', function() {
      carousel.prev()
    })

  document
    .querySelector('.g-carousel .next')
    .addEventListener('click', function() {
      carousel.next()
    })

  for (let i = 0; i < dots.length; i++) {
    dots[i].addEventListener('click', function() {
      carousel.goTo(i)
    })
  }
}
