document.addEventListener('turbolinks:load', function () {
  var qs = document.querySelector.bind(document)
  var qsa = document.querySelectorAll.bind(document)
  var carousel = qs('.siema')

  if (carousel) {
    objectFitImages()

    // siema carousels
    var dots = qsa('.g-carousel .pagination li')
    var siema = new Siema({
      selector: carousel,
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
        dots[siema.currentSlide].classList.add('active')
      }
    })

    // on previous button click
    qs('.g-carousel .prev')
      .addEventListener('click', function() {
        siema.prev()
      })
  
    // on next button click
    qs('.g-carousel .next')
      .addEventListener('click', function() {
        siema.next()
      })
  
    // on dot click
    for (var i = 0; i < dots.length; i++) {
      dots[i].addEventListener('click', function() {
        siema.goTo(this.dataset.index)
      })
    }

    document.addEventListener('turbolinks:before-cache', function() {
      siema.goTo(0) // reset pagination
      siema.destroy(true)
    });
  }  
})

