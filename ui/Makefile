server:
	python -m SimpleHTTPServer

watch:
	sass styles:static --watch

deps:
	go get github.com/jteeuwen/go-bindata/...
	go get github.com/elazarl/go-bindata-assetfs/...
	bundler install

dist:
	@echo clean dist
	@rm -rf dist/index.html
	@rm -rf dist/static
	@echo "compile styles/*.scss"
	@sass styles/base.scss static/base.css
	@ruby scripts/compile.rb
	cp -R ./static dist/static/
	cp index.html dist/index.html
	sed -E -e "/ASSETS/,/\/ASSETS/ d" -ibak dist/index.html
	sed -E -e "s#<\/body>#<script src=\"static/application.min.js\"></script></body>#" -ibak dist/index.html
	rm dist/index.htmlbak
	go-bindata-assetfs -ignore '\.git.+' -pkg agent dist/...
	mv bindata_assetfs.go ../command/agent/

.PHONY: server watch deps dist
