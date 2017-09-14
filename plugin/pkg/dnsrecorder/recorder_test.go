package dnsrecorder

/*
func TestNewResponseRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	recordRequest := NewResponseRecorder(w)
	if !(recordRequest.ResponseWriter == w) {
		t.Fatalf("Expected Response writer in the Recording to be same as the one sent\n")
	}
	if recordRequest.status != http.StatusOK {
		t.Fatalf("Expected recorded status  to be http.StatusOK (%d) , but found %d\n ", http.StatusOK, recordRequest.status)
	}
}

func TestWrite(t *testing.T) {
	w := httptest.NewRecorder()
	responseTestString := "test"
	recordRequest := NewResponseRecorder(w)
	buf := []byte(responseTestString)
	recordRequest.Write(buf)
	if recordRequest.size != len(buf) {
		t.Fatalf("Expected the bytes written counter to be %d, but instead found %d\n", len(buf), recordRequest.size)
	}
	if w.Body.String() != responseTestString {
		t.Fatalf("Expected Response Body to be %s , but found %s\n", responseTestString, w.Body.String())
	}
}
*/
