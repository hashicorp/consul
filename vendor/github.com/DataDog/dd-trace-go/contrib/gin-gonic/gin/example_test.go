package gin_test

import (
	gintrace "github.com/DataDog/dd-trace-go/contrib/gin-gonic/gin"
	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/gin-gonic/gin"
)

// To start tracing requests, add the trace middleware to your Gin router.
func Example() {
	// Create your router and use the middleware.
	r := gin.New()
	r.Use(gintrace.Middleware("my-web-app"))

	r.GET("/hello", func(c *gin.Context) {
		c.String(200, "hello world!")
	})

	// Profit!
	r.Run(":8080")
}

func ExampleHTML() {
	r := gin.Default()
	r.Use(gintrace.Middleware("my-web-app"))
	r.LoadHTMLGlob("templates/*")

	r.GET("/index", func(c *gin.Context) {
		// This will render the html and trace the execution time.
		gintrace.HTML(c, 200, "index.tmpl", gin.H{
			"title": "Main website",
		})
	})
}

func ExampleSpanDefault() {
	r := gin.Default()
	r.Use(gintrace.Middleware("image-encoder"))

	r.GET("/image/encode", func(c *gin.Context) {
		// The middleware patches a span to the request. Let's add some metadata,
		// and create a child span.
		span := gintrace.SpanDefault(c)
		span.SetMeta("user.handle", "admin")
		span.SetMeta("user.id", "1234")

		encodeSpan := tracer.NewChildSpan("image.encode", span)
		// encode a image
		encodeSpan.Finish()

		uploadSpan := tracer.NewChildSpan("image.upload", span)
		// upload the image
		uploadSpan.Finish()

		c.String(200, "ok!")
	})

}
