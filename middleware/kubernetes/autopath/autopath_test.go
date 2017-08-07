package autopath

import "testing"

func TestSplitSearchPath(t *testing.T) {
	type testCase struct {
		question       string
		namespace      string
		expectedName   string
		expectedSearch string
		expectedOk     bool
	}
	tests := []testCase{
		{question: "test.blah.com", namespace: "ns1", expectedName: "", expectedSearch: "", expectedOk: false},
		{question: "foo.com.ns2.svc.interwebs.nets", namespace: "ns1", expectedName: "", expectedSearch: "", expectedOk: false},
		{question: "foo.com.svc.interwebs.nets", namespace: "ns1", expectedName: "", expectedSearch: "", expectedOk: false},
		{question: "foo.com.ns1.svc.interwebs.nets", namespace: "ns1", expectedName: "foo.com", expectedSearch: "ns1.svc.interwebs.nets", expectedOk: true},
	}
	zone := "interwebs.nets"
	for _, c := range tests {
		name, search, ok := SplitSearch(zone, c.question, c.namespace)
		if c.expectedName != name || c.expectedSearch != search || c.expectedOk != ok {
			t.Errorf("Case %v: Expected name'%v', search:'%v', ok:'%v'. Got name:'%v', search:'%v', ok:'%v'.", c.question, c.expectedName, c.expectedSearch, c.expectedOk, name, search, ok)
		}
	}
}
