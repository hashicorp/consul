package restful

import (
	"io"
	"net/http"
	"reflect"
	"sort"
	"testing"
)

//
// Step 1 tests
//
var paths = []struct {
	// url with path (1) is handled by service with root (2) and last capturing group has value final (3)
	path, root, final string
	params            map[string]string
}{
	{"/", "/", "/", map[string]string{}},
	{"/p", "/p", "", map[string]string{}},
	{"/p/x", "/p/{q}", "", map[string]string{"q": "x"}},
	{"/q/x", "/q", "/x", map[string]string{}},
	{"/p/x/", "/p/{q}", "/", map[string]string{"q": "x"}},
	{"/p/x/y", "/p/{q}", "/y", map[string]string{"q": "x"}},
	{"/q/x/y", "/q", "/x/y", map[string]string{}},
	{"/z/q", "/{p}/q", "", map[string]string{"p": "z"}},
	{"/a/b/c/q", "/", "/a/b/c/q", map[string]string{}},
}

func TestDetectDispatcher(t *testing.T) {
	ws1 := new(WebService).Path("/")
	ws2 := new(WebService).Path("/p")
	ws3 := new(WebService).Path("/q")
	ws4 := new(WebService).Path("/p/q")
	ws5 := new(WebService).Path("/p/{q}")
	ws6 := new(WebService).Path("/p/{q}/")
	ws7 := new(WebService).Path("/{p}/q")
	var dispatchers = []*WebService{ws1, ws2, ws3, ws4, ws5, ws6, ws7}

	wc := NewContainer()
	for _, each := range dispatchers {
		each.Route(each.GET("").To(dummy))
		wc.Add(each)
	}

	router := RouterJSR311{}

	ok := true
	for i, fixture := range paths {
		who, final, err := router.detectDispatcher(fixture.path, dispatchers)
		if err != nil {
			t.Logf("error in detection:%v", err)
			ok = false
		}
		if who.RootPath() != fixture.root {
			t.Logf("[line:%v] Unexpected dispatcher, expected:%v, actual:%v", i, fixture.root, who.RootPath())
			ok = false
		}
		if final != fixture.final {
			t.Logf("[line:%v] Unexpected final, expected:%v, actual:%v", i, fixture.final, final)
			ok = false
		}
		params := router.ExtractParameters(&who.Routes()[0], who, fixture.path)
		if !reflect.DeepEqual(params, fixture.params) {
			t.Logf("[line:%v] Unexpected params, expected:%v, actual:%v", i, fixture.params, params)
			ok = false
		}
	}
	if !ok {
		t.Fail()
	}
}

//
// Step 2 tests
//

// go test -v -test.run TestISSUE_179 ...restful
func TestISSUE_179(t *testing.T) {
	ws1 := new(WebService)
	ws1.Route(ws1.GET("/v1/category/{param:*}").To(dummy))
	routes := RouterJSR311{}.selectRoutes(ws1, "/v1/category/sub/sub")
	t.Logf("%v", routes)
}

// go test -v -test.run TestISSUE_30 ...restful
func TestISSUE_30(t *testing.T) {
	ws1 := new(WebService).Path("/users")
	ws1.Route(ws1.GET("/{id}").To(dummy))
	ws1.Route(ws1.POST("/login").To(dummy))
	routes := RouterJSR311{}.selectRoutes(ws1, "/login")
	if len(routes) != 2 {
		t.Fatal("expected 2 routes")
	}
	if routes[0].Path != "/users/login" {
		t.Error("first is", routes[0].Path)
		t.Logf("routes:%v", routes)
	}
}

// go test -v -test.run TestISSUE_34 ...restful
func TestISSUE_34(t *testing.T) {
	ws1 := new(WebService).Path("/")
	ws1.Route(ws1.GET("/{type}/{id}").To(dummy))
	ws1.Route(ws1.GET("/network/{id}").To(dummy))
	routes := RouterJSR311{}.selectRoutes(ws1, "/network/12")
	if len(routes) != 2 {
		t.Fatal("expected 2 routes")
	}
	if routes[0].Path != "/network/{id}" {
		t.Error("first is", routes[0].Path)
		t.Logf("routes:%v", routes)
	}
}

// go test -v -test.run TestISSUE_34_2 ...restful
func TestISSUE_34_2(t *testing.T) {
	ws1 := new(WebService).Path("/")
	// change the registration order
	ws1.Route(ws1.GET("/network/{id}").To(dummy))
	ws1.Route(ws1.GET("/{type}/{id}").To(dummy))
	routes := RouterJSR311{}.selectRoutes(ws1, "/network/12")
	if len(routes) != 2 {
		t.Fatal("expected 2 routes")
	}
	if routes[0].Path != "/network/{id}" {
		t.Error("first is", routes[0].Path)
	}
}

// go test -v -test.run TestISSUE_137 ...restful
func TestISSUE_137(t *testing.T) {
	ws1 := new(WebService)
	ws1.Route(ws1.GET("/hello").To(dummy))
	routes := RouterJSR311{}.selectRoutes(ws1, "/")
	t.Log(routes)
	if len(routes) > 0 {
		t.Error("no route expected")
	}
}

func TestSelectRoutesSlash(t *testing.T) {
	ws1 := new(WebService).Path("/")
	ws1.Route(ws1.GET("").To(dummy))
	ws1.Route(ws1.GET("/").To(dummy))
	ws1.Route(ws1.GET("/u").To(dummy))
	ws1.Route(ws1.POST("/u").To(dummy))
	ws1.Route(ws1.POST("/u/v").To(dummy))
	ws1.Route(ws1.POST("/u/{w}").To(dummy))
	ws1.Route(ws1.POST("/u/{w}/z").To(dummy))
	routes := RouterJSR311{}.selectRoutes(ws1, "/u")
	checkRoutesContains(routes, "/u", t)
	checkRoutesContainsNo(routes, "/u/v", t)
	checkRoutesContainsNo(routes, "/", t)
	checkRoutesContainsNo(routes, "/u/{w}/z", t)
}
func TestSelectRoutesU(t *testing.T) {
	ws1 := new(WebService).Path("/u")
	ws1.Route(ws1.GET("").To(dummy))
	ws1.Route(ws1.GET("/").To(dummy))
	ws1.Route(ws1.GET("/v").To(dummy))
	ws1.Route(ws1.POST("/{w}").To(dummy))
	ws1.Route(ws1.POST("/{w}/z").To(dummy))          // so full path = /u/{w}/z
	routes := RouterJSR311{}.selectRoutes(ws1, "/v") // test against /u/v
	checkRoutesContains(routes, "/u/{w}", t)
}

func TestSelectRoutesUsers1(t *testing.T) {
	ws1 := new(WebService).Path("/users")
	ws1.Route(ws1.POST("").To(dummy))
	ws1.Route(ws1.POST("/").To(dummy))
	ws1.Route(ws1.PUT("/{id}").To(dummy))
	routes := RouterJSR311{}.selectRoutes(ws1, "/1")
	checkRoutesContains(routes, "/users/{id}", t)
}
func checkRoutesContains(routes []Route, path string, t *testing.T) {
	if !containsRoutePath(routes, path, t) {
		for _, r := range routes {
			t.Logf("route %v %v", r.Method, r.Path)
		}
		t.Fatalf("routes should include [%v]:", path)
	}
}
func checkRoutesContainsNo(routes []Route, path string, t *testing.T) {
	if containsRoutePath(routes, path, t) {
		for _, r := range routes {
			t.Logf("route %v %v", r.Method, r.Path)
		}
		t.Fatalf("routes should not include [%v]:", path)
	}
}
func containsRoutePath(routes []Route, path string, t *testing.T) bool {
	for _, each := range routes {
		if each.Path == path {
			return true
		}
	}
	return false
}

// go test -v -test.run TestSortableRouteCandidates ...restful
func TestSortableRouteCandidates(t *testing.T) {
	fixture := &sortableRouteCandidates{}
	r1 := routeCandidate{matchesCount: 0, literalCount: 0, nonDefaultCount: 0}
	r2 := routeCandidate{matchesCount: 0, literalCount: 0, nonDefaultCount: 1}
	r3 := routeCandidate{matchesCount: 0, literalCount: 1, nonDefaultCount: 1}
	r4 := routeCandidate{matchesCount: 1, literalCount: 1, nonDefaultCount: 0}
	r5 := routeCandidate{matchesCount: 1, literalCount: 0, nonDefaultCount: 0}
	fixture.candidates = append(fixture.candidates, r5, r4, r3, r2, r1)
	sort.Sort(sort.Reverse(fixture))
	first := fixture.candidates[0]
	if first.matchesCount != 1 && first.literalCount != 1 && first.nonDefaultCount != 0 {
		t.Fatal("expected r4")
	}
	last := fixture.candidates[len(fixture.candidates)-1]
	if last.matchesCount != 0 && last.literalCount != 0 && last.nonDefaultCount != 0 {
		t.Fatal("expected r1")
	}
}

func TestDetectRouteReturns404IfNoRoutePassesConditions(t *testing.T) {
	called := false
	shouldNotBeCalledButWas := false

	routes := []Route{
		new(RouteBuilder).To(dummy).
			If(func(req *http.Request) bool { return false }).
			Build(),

		// check that condition functions are called in order
		new(RouteBuilder).
			To(dummy).
			If(func(req *http.Request) bool { return true }).
			If(func(req *http.Request) bool { called = true; return false }).
			Build(),

		// check that condition functions short circuit
		new(RouteBuilder).
			To(dummy).
			If(func(req *http.Request) bool { return false }).
			If(func(req *http.Request) bool { shouldNotBeCalledButWas = true; return false }).
			Build(),
	}

	_, err := RouterJSR311{}.detectRoute(routes, (*http.Request)(nil))
	if se := err.(ServiceError); se.Code != 404 {
		t.Fatalf("expected 404, got %d", se.Code)
	}

	if !called {
		t.Fatal("expected condition function to get called, but it wasn't")
	}

	if shouldNotBeCalledButWas {
		t.Fatal("expected condition function to not be called, but it was")
	}
}

var extractParams = []struct {
	name           string
	routePath      string
	urlPath        string
	expectedParams map[string]string
}{
	{"wildcardLastPart", "/fixed/{var:*}", "/fixed/remainder", map[string]string{"var": "remainder"}},
	{"wildcardMultipleParts", "/fixed/{var:*}", "/fixed/remain/der", map[string]string{"var": "remain/der"}},
	{"wildcardManyParts", "/fixed/{var:*}", "/fixed/test/sub/hi.html", map[string]string{"var": "test/sub/hi.html"}},
	{"wildcardInMiddle", "/fixed/{var:*}/morefixed", "/fixed/middle/stuff/morefixed", map[string]string{"var": "middle/stuff"}},
	{"wildcardFollowedByVar", "/fixed/{var:*}/morefixed/{otherVar}", "/fixed/middle/stuff/morefixed/end", map[string]string{"var": "middle/stuff", "otherVar": "end"}},
	{"singleParam", "/fixed/{var}", "/fixed/remainder", map[string]string{"var": "remainder"}},
	{"slash", "/", "/", map[string]string{}},
	{"NoVars", "/fixed", "/fixed", map[string]string{}},
	{"TwoVars", "/from/{source}/to/{destination}", "/from/LHR/to/AMS", map[string]string{"source": "LHR", "destination": "AMS"}},
	{"VarOnFront", "/{what}/from/{source}", "/who/from/SOS", map[string]string{"what": "who", "source": "SOS"}},
}

func TestExtractParams(t *testing.T) {
	for _, testCase := range extractParams {
		t.Run(testCase.name, func(t *testing.T) {
			ws1 := new(WebService).Path("/")
			ws1.Route(ws1.GET(testCase.routePath).To(dummy))
			router := RouterJSR311{}
			req, _ := http.NewRequest(http.MethodGet, testCase.urlPath, nil)
			params := router.ExtractParameters(&ws1.Routes()[0], ws1, req.URL.Path)
			if len(params) != len(testCase.expectedParams) {
				t.Fatalf("Wrong length of params on selected route, expected: %v, got: %v", testCase.expectedParams, params)
			}
			for expectedParamKey, expectedParamValue := range testCase.expectedParams {
				if expectedParamValue != params[expectedParamKey] {
					t.Errorf("Wrong parameter for key '%v', expected: %v, got: %v", expectedParamKey, expectedParamValue, params[expectedParamKey])
				}
			}
		})
	}
}

func TestSelectRouteInvalidMethod(t *testing.T) {
	ws1 := new(WebService).Path("/")
	ws1.Route(ws1.GET("/simple").To(dummy))
	router := RouterJSR311{}
	req, _ := http.NewRequest(http.MethodPost, "/simple", nil)
	_, _, err := router.SelectRoute([]*WebService{ws1}, req)
	if err == nil {
		t.Fatal("Expected an error as the wrong method is used but was nil")
	}
}

func TestParameterInWebService(t *testing.T) {
	for _, testCase := range extractParams {
		t.Run(testCase.name, func(t *testing.T) {
			ws1 := new(WebService).Path("/{wsParam}")
			ws1.Route(ws1.GET(testCase.routePath).To(dummy))
			router := RouterJSR311{}
			req, _ := http.NewRequest(http.MethodGet, "/wsValue"+testCase.urlPath, nil)
			params := router.ExtractParameters(&ws1.Routes()[0], ws1, req.URL.Path)
			expectedParams := map[string]string{"wsParam": "wsValue"}
			for key, value := range testCase.expectedParams {
				expectedParams[key] = value
			}
			if len(params) != len(expectedParams) {
				t.Fatalf("Wrong length of params on selected route, expected: %v, got: %v", testCase.expectedParams, params)
			}
			for expectedParamKey, expectedParamValue := range testCase.expectedParams {
				if expectedParamValue != params[expectedParamKey] {
					t.Errorf("Wrong parameter for key '%v', expected: %v, got: %v", expectedParamKey, expectedParamValue, params[expectedParamKey])
				}
			}
		})
	}
}

func dummy(req *Request, resp *Response) { io.WriteString(resp.ResponseWriter, "dummy") }
