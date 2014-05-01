server:
	python -m SimpleHTTPServer

watch:
	sass styles:static --watch

dist:
	@echo "compile styles/*.scss"
	@sass styles/base.scss static/base.css
	@ruby scripts/compile.rb
	cp -R ./static dist/static
	cp index.html dist/index.html

.PHONY: server watch dist
