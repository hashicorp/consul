package witch

import "net/http"
import (
	"bytes"
	"github.com/rakyll/statik/fs"
	"github.com/v2pro/plz/countlog"
	_ "github.com/v2pro/plz/witch/statik"
	"io/ioutil"
	"github.com/v2pro/plz/countlog/output"
	"github.com/v2pro/plz/countlog/output/json"
	"expvar"
	"path"
	"runtime"
	"reflect"
)

var files = []string{
	"ide.html",
	"log-viewer.html", "filters.html", "data-sources.html", "columns.html",
	"state-viewer.html", "snapshots.html",
	"viz.html", "viz-item.html", "viz-value.html",
	"viz-struct.html", "viz-array.html",
	"viz-plain.html", "viz-ptr.html"}

//go:generate $GOPATH/bin/statik -src $PWD/webroot

var viewerHtml []byte

var Mux = &http.ServeMux{}

func init() {
	Mux.HandleFunc("/witch/more-events", moreEvents)
	Mux.Handle("/witch/expvar", expvar.Handler())
	Mux.HandleFunc("/witch/", homepage)
}

func initViewerHtml() error {
	if viewerHtml != nil {
		return nil
	}
	statikFS, err := fs.New()
	if err != nil {
		countlog.Error("event!witch.failed to load witch viewer web resource", "err", err)
		return err
	}
	indexHtmlFile, err := statikFS.Open("/index.html")
	if err != nil {
		countlog.Error("event!witch.failed to open index.html", "err", err)
		return err
	}
	indexHtml, err := ioutil.ReadAll(indexHtmlFile)
	if err != nil {
		countlog.Error("event!witch.failed to read index.html", "err", err)
		return err
	}
	components := []byte{}
	for _, file := range files {
		f, err := statikFS.Open("/" + file)
		if err != nil {
			countlog.Error("event!witch.failed to open file", "err", err, "file", file)
			return err
		}
		fileHtml, err := ioutil.ReadAll(f)
		if err != nil {
			countlog.Error("event!witch.failed to read file html", "err", err, "file", file)
			return err
		}
		components = append(components, fileHtml...)
	}
	viewerHtml = bytes.Replace(indexHtml, []byte("{{ COMPONENTS }}"), components, -1)
	return nil
}

var isDebug = true
var thisFile string

func init() {
	pc := reflect.ValueOf(loadDebugViewerHtml).Pointer()
	thisFile, _ = runtime.FuncForPC(pc).FileLine(pc)
}

func loadDebugViewerHtml() ([]byte, error) {
	dir := path.Join(path.Dir(thisFile), "webroot")
	indexHtml, err := ioutil.ReadFile(path.Join(dir, "index.html"))
	if err != nil {
		countlog.Error("event!witch.failed to read index.html", "err", err)
		return nil, err
	}
	components := []byte{}
	for _, file := range files {
		fileHtml, err := ioutil.ReadFile(path.Join(dir, file))
		if err != nil {
			countlog.Error("event!witch.failed to read file", "err", err, "file", file)
			return nil, err
		}
		components = append(components, fileHtml...)
	}
	components = append(components, `
	<script>
		window.isDebug = true;
	</script>
	`...)
	return bytes.Replace(indexHtml, []byte("{{ COMPONENTS }}"), components, -1), nil
}

func Start(addr string) {
	err := initViewerHtml()
	if err != nil {
		countlog.Error("event!witch.failed to init viewer html", "err", err)
		return
	}
	countlog.Info("event!witch.viewer started", "addr", addr)
	countlog.EventWriter = output.NewEventWriter(output.EventWriterConfig{
		Format: &json.Format{},
		Writer: theEventQueue,
	})
	if addr != "" {
		go func() {
			setCurrentGoRoutineIsKoala()
			http.ListenAndServe(addr, Mux)
		}()
	}
}

func homepage(respWriter http.ResponseWriter, req *http.Request) {
	setCurrentGoRoutineIsKoala()
	if !isDebug {
		respWriter.Write(viewerHtml)
		return
	}
	debugViewerHtml, err := loadDebugViewerHtml()
	if err != nil {
		respWriter.Write([]byte(err.Error()))
	} else {
		respWriter.Write(debugViewerHtml)
	}
}
