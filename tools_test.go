package Toolbox_Reusable_Go_module

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

// RoundTripFunc is used to satisfy the interface requirements for http.Client
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip is used to satisfy the interface requirements for http.Client
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

type testData struct {
	Data any `json:"bar"`
}

var pushTests = []struct {
	name          string
	payload       any
	errorExpected bool
}{
	{
		name: "valid",
		payload: testData{
			Data: "bar",
		},
		errorExpected: false,
	},
	{
		name:          "invalid",
		payload:       make(chan int),
		errorExpected: true,
	},
}

func TestNew(t *testing.T) {
	tools := New()
	if tools.MaxXMLSize != defaultMaxUpload {
		t.Error("wrong MaxXMLSize")
	}
}

func TestTools_PushJSONToRemote(t *testing.T) {
	for _, e := range pushTests {
		client := NewTestClient(func(req *http.Request) *http.Response {
			// Test request parameters
			return &http.Response{
				StatusCode: http.StatusOK,
				// Send response to be tested
				Body: io.NopCloser(bytes.NewBufferString(`OK`)),
				// Must be set to non-nil value or it panics
				Header: make(http.Header),
			}
		})

		var testTools Tools

		_, _, err := testTools.PushJSONToRemote("http://example.com/some/path", e.payload, client)
		if err == nil && e.errorExpected {
			t.Errorf("%s: error expected, but none received", e.name)
		}

		if err != nil && !e.errorExpected {
			t.Errorf("%s: no error expected, but one received: %v", e.name, err)
		}
	}
}

var jsonTests = []struct {
	name          string
	json          string
	errorExpected bool
	maxSize       int
	allowUnknown  bool
	contentType   string
}{
	{name: "good json", json: `{"foo": "bar"}`, errorExpected: false, maxSize: 1024, allowUnknown: false},
	{name: "badly formatted json", json: `{"foo":"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "incorrect type", json: `{"foo": 1}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "incorrect type", json: `{1: 1}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "two json files", json: `{"foo": "bar"}{"alpha": "beta"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "empty body", json: ``, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "syntax error in json", json: `{"foo": 1"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "unknown field in json", json: `{"fooo": "bar"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "incorrect type for field", json: `{"foo": 10.2}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "allow unknown field in json", json: `{"fooo": "bar"}`, errorExpected: false, maxSize: 1024, allowUnknown: true},
	{name: "missing field name", json: `{jack: "bar"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "file too large", json: `{"foo": "bar"}`, errorExpected: true, maxSize: 5, allowUnknown: false},
	{name: "not json", json: `Hello, world`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "wrong header", json: `{"foo": "bar"}`, errorExpected: true, maxSize: 1024, allowUnknown: false, contentType: "application/xml"},
}

func TestTools_ReadJSON(t *testing.T) {
	for _, e := range jsonTests {
		var testTools Tools
		// set max file size
		testTools.MaxJSONSize = e.maxSize

		// allow/disallow unknown fields.
		testTools.AllowUnknownFields = e.allowUnknown

		// declare a variable to read the decoded json into.
		var decodedJSON struct {
			Foo string `json:"foo"`
		}

		// create a request with the body.
		req, err := http.NewRequest("POST", "/", bytes.NewReader([]byte(e.json)))
		if err != nil {
			t.Log("Error", err)
		}
		if e.contentType != "" {
			req.Header.Add("Content-Type", e.contentType)
		} else {
			req.Header.Add("Content-Type", "application/json")
		}

		// create a test response recorder, which satisfies the requirements
		// for a ResponseWriter.
		rr := httptest.NewRecorder()

		// call ReadJSON and check for an error.
		err = testTools.ReadJSON(rr, req, &decodedJSON)

		// if we expect an error, but do not get one, something went wrong.
		if e.errorExpected && err == nil {
			t.Errorf("%s: error expected, but none received", e.name)
		}

		// if we do not expect an error, but get one, something went wrong.
		if !e.errorExpected && err != nil {
			t.Errorf("%s: error not expected, but one received: %s \n%s", e.name, err.Error(), e.json)
		}
		req.Body.Close()
	}
}

func TestTools_ReadJSONAndMarshal(t *testing.T) {
	// set max file size
	var testTools Tools

	// create a request with the body
	req, err := http.NewRequest("POST", "/", bytes.NewReader([]byte(`{"foo": "bar"}`)))
	if err != nil {
		t.Log("Error", err)
	}

	// create a test response recorder, which satisfies the requirements
	// for a ResponseWriter
	rr := httptest.NewRecorder()

	// call readJSON and check for an error; since we are using nil for the final parameter,
	// we should get an error
	err = testTools.ReadJSON(rr, req, nil)

	// we expect an error, but did not get one, so something went wrong
	if err == nil {
		t.Error("error expected, but none received")
	}

	req.Body.Close()
}

var writeJSONTests = []struct {
	name          string
	payload       any
	errorExpected bool
}{
	{
		name: "valid",
		payload: JSONResponse{
			Error:   false,
			Message: "foo",
		},
		errorExpected: false,
	},
	{
		name:          "invalid",
		payload:       make(chan int),
		errorExpected: true,
	},
}

func TestTools_WriteJSON(t *testing.T) {
	for _, e := range writeJSONTests {
		// create a variable of type toolbox.Tools, and just use the defaults.
		var testTools Tools

		rr := httptest.NewRecorder()

		headers := make(http.Header)
		headers.Add("FOO", "BAR")
		err := testTools.WriteJSON(rr, http.StatusOK, e.payload, headers)
		if err == nil && e.errorExpected {
			t.Errorf("%s: expected error, but did not get one", e.name)
		}
		if err != nil && !e.errorExpected {
			t.Errorf("%s: did not expect error, but got one: %v", e.name, err)
		}
	}
}

func TestTools_ErrorJSON(t *testing.T) {
	var testTools Tools

	rr := httptest.NewRecorder()
	err := testTools.ErrorJSON(rr, errors.New("some error"), http.StatusServiceUnavailable)
	if err != nil {
		t.Error(err)
	}

	var requestPayload JSONResponse
	decoder := json.NewDecoder(rr.Body)
	err = decoder.Decode(&requestPayload)
	if err != nil {
		t.Error("received error when decoding ErrorJSON payload:", err)
	}

	if !requestPayload.Error {
		t.Error("error set to false in response from ErrorJSON, and should be set to true")
	}

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("wrong status code returned; expected 503, but got %d", rr.Code)
	}
}

func TestTools_RandomString(t *testing.T) {
	var testTools Tools

	s := testTools.RandomString(10)
	if len(s) != 10 {
		t.Error("wrong length random string returned")
	}
}

func TestTools_DownloadLargeStaticFile(t *testing.T) {
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	var testTools Tools

	testTools.DownloadStaticFile(rr, req, "./testdata", "tgg.jpg", "gatsby.jpg")

	res := rr.Result()
	defer res.Body.Close()

	if res.Header["Content-Length"][0] != "240366" {
		t.Error("wrong content length of", res.Header["Content-Length"][0])
	}

	if res.Header["Content-Disposition"][0] != "attachment; filename=\"gatsby.jpg\"" {
		t.Error("wrong content disposition of", res.Header["Content-Disposition"][0])
	}

	_, err := io.ReadAll(res.Body)
	if err != nil {
		t.Error(err)
	}
}

var uploadTests = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
	maxSize       int
	uploadDir     string
}{
	{name: "allowed no rename", allowedTypes: []string{"image/jpeg", "image/png"}, renameFile: false, errorExpected: false, maxSize: 0, uploadDir: ""},
	{name: "allowed rename", allowedTypes: []string{"image/jpeg", "image/png"}, renameFile: true, errorExpected: false, maxSize: 0, uploadDir: ""},
	{name: "allowed no filetype specified", allowedTypes: []string{}, renameFile: true, errorExpected: false, maxSize: 0, uploadDir: ""},
	{name: "not allowed", allowedTypes: []string{"image/jpeg"}, errorExpected: true, maxSize: 0, uploadDir: ""},
	{name: "too big", allowedTypes: []string{"image/jpeg,", "image/png"}, errorExpected: true, maxSize: 10, uploadDir: ""},
	{name: "invalid directory", allowedTypes: []string{"image/jpeg,", "image/png"}, errorExpected: true, maxSize: 0, uploadDir: "//"},
}

func TestTools_UploadFiles(t *testing.T) {
	for _, e := range uploadTests {
		// set up a pipe to avoid buffering
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer writer.Close()
			defer wg.Done()

			// create the form data field 'file'
			part, err := writer.CreateFormFile("file", "./testdata/img.png")
			if err != nil {
				t.Error(err)
			}

			f, err := os.Open("./testdata/img.png")
			if err != nil {
				t.Error(err)
			}
			defer f.Close()
			img, _, err := image.Decode(f)
			if err != nil {
				t.Error("error decoding image", err)
			}

			err = png.Encode(part, img)
			if err != nil {
				t.Error(err)
			}
		}()

		// read from the pipe which receives data
		request := httptest.NewRequest("POST", "/", pr)
		request.Header.Add("Content-Type", writer.FormDataContentType())

		var testTools Tools
		testTools.AllowedFileTypes = e.allowedTypes
		if e.maxSize > 0 {
			testTools.MaxFileSize = e.maxSize
		}

		var uploadDir = "./testdata/uploads/"
		if e.uploadDir != "" {
			uploadDir = e.uploadDir
		}

		uploadedFiles, err := testTools.UploadFiles(request, uploadDir, e.renameFile)
		if err != nil && !e.errorExpected {
			t.Error(err)
		}

		if !e.errorExpected {
			if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName)); os.IsNotExist(err) {
				t.Errorf("%s: expected file to exist: %s", e.name, err.Error())
			}

			// clean up
			_ = os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName))
		}

		if e.errorExpected && err == nil {
			t.Errorf("%s: error expected, but none received", e.name)
		}

		// we're running table tests, so have to use a waitgroup
		wg.Wait()
	}
}

var uploadOneTests = []struct {
	name          string
	uploadDir     string
	errorExpected bool
}{
	{name: "valid", uploadDir: "./testdata/uploads/", errorExpected: false},
	{name: "invalid", uploadDir: "//", errorExpected: true},
}

func TestTools_UploadOneFile(t *testing.T) {
	for _, e := range uploadOneTests {
		// set up a pipe to avoid buffering
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)

		go func() {
			defer writer.Close()

			// create the form data field 'file'
			part, err := writer.CreateFormFile("file", "./testdata/img.png")
			if err != nil {
				t.Error(err)
			}

			f, err := os.Open("./testdata/img.png")
			if err != nil {
				t.Error(err)
			}
			defer f.Close()
			img, _, err := image.Decode(f)
			if err != nil {
				t.Error("error decoding image", err)
			}

			err = png.Encode(part, img)
			if err != nil {
				t.Error(err)
			}
		}()

		// read from the pipe which receives data
		request := httptest.NewRequest("POST", "/", pr)
		request.Header.Add("Content-Type", writer.FormDataContentType())

		var testTools Tools
		testTools.AllowedFileTypes = []string{"image/png"}

		uploadedFiles, err := testTools.UploadOneFile(request, e.uploadDir, true)
		if e.errorExpected && err == nil {
			t.Errorf("%s: error expected, but none received", e.name)
		}

		if !e.errorExpected {
			if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles.NewFileName)); os.IsNotExist(err) {
				t.Errorf("%s: expected file to exist: %s", e.name, err.Error())
			}

			// clean up
			_ = os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles.NewFileName))
		}
	}
}

func TestTools_CreateDirIfNotExist(t *testing.T) {
	var testTool Tools

	err := testTool.CreateDirIfNotExist("./testdata/myDir")
	if err != nil {
		t.Error(err)
	}

	err = testTool.CreateDirIfNotExist("./testdata/myDir")
	if err != nil {
		t.Error(err)
	}

	_ = os.Remove("./testdata/myDir")
}

func TestTools_CreateDirIfNotExistInvalidDirectory(t *testing.T) {
	var testTool Tools

	// we should not be able to create a directory at the root level (no permissions)
	err := testTool.CreateDirIfNotExist("/mydir")
	if err == nil {
		t.Error(errors.New("able to create a directory where we should not be able to"))
	}
}

var slugTests = []struct {
	name          string
	s             string
	expected      string
	errorExpected bool
}{
	{name: "valid string", s: "now is the time", expected: "now-is-the-time", errorExpected: false},
	{name: "empty string", s: "", expected: "", errorExpected: true},
	{name: "complex string", s: "Now is the time for all GOOD men! + Fish & such &^?123", expected: "now-is-the-time-for-all-good-men-fish-such-123", errorExpected: false},
	{name: "japanese string", s: "こんにちは世界", expected: "", errorExpected: true},
	{name: "japanese string plus roman characters", s: "こんにちは世界 hello world", expected: "hello-world", errorExpected: false},
}

func TestTools_Slugify(t *testing.T) {
	var testTool Tools

	for _, e := range slugTests {
		slug, err := testTool.Slugify(e.s)
		if err != nil && !e.errorExpected {
			t.Errorf("%s: error received when none expected: %s", e.name, err.Error())
		}

		if !e.errorExpected && slug != e.expected {
			t.Errorf("%s: wrong slug returned; expected %s but got %s", e.name, e.expected, slug)
		}
	}
}

var writeXMLTests = []struct {
	name          string
	payload       any
	errorExpected bool
}{
	{
		name: "valid",
		payload: XMLResponse{
			Error:   false,
			Message: "foo",
		},
		errorExpected: false,
	},
	{
		name:          "invalid",
		payload:       make(chan int),
		errorExpected: true,
	},
}

func TestTools_WriteXML(t *testing.T) {
	for _, e := range writeXMLTests {
		// create a variable of type toolbox.Tools, and just use the defaults.
		var testTools Tools

		rr := httptest.NewRecorder()

		headers := make(http.Header)
		headers.Add("FOO", "BAR")
		err := testTools.WriteXML(rr, http.StatusOK, e.payload, headers)
		if err != nil && !e.errorExpected {
			t.Errorf("%s, failed to write XML: %v", e.name, err)
		}

		if err == nil && e.errorExpected {
			t.Errorf("%s: error expected, but none received", e.name)
		}
	}
}

var xmlTests = []struct {
	name          string
	xml           string
	maxBytes      int
	errorExpected bool
}{
	{
		name:          "good xml",
		xml:           `<?xml version="1.0" encoding="UTF-8"?><note><to>John Smith</to><from>Jane Jones</from></note>`,
		errorExpected: false,
	},
	{
		name:          "badly formatted xml",
		xml:           `<?xml version="1.0" encoding="UTF-8"?><note><xx>John Smith</to><from>Jane Jones</from></note>`,
		errorExpected: true,
	},
	{
		name:          "too big",
		xml:           `<?xml version="1.0" encoding="UTF-8"?><note><to>John Smith</to><from>Jane Jones</from></note>`,
		maxBytes:      10,
		errorExpected: true,
	},
	{
		name: "double xml",
		xml: `<?xml version="1.0" encoding="UTF-8"?><note><to>John Smith</to><from>Jane Jones</from></note>
						<?xml version="1.0" encoding="UTF-8"?><note><to>Luke Skywalker</to><from>R2D2</from></note>`,
		errorExpected: true,
	},
}

func TestTools_ReadXML(t *testing.T) {
	for _, e := range xmlTests {
		// create a variable of type toolbox.Tools, and just use the defaults.
		var tools Tools

		if e.maxBytes != 0 {
			tools.MaxXMLSize = e.maxBytes
		}
		// create a request with the body.
		req, err := http.NewRequest("POST", "/", bytes.NewReader([]byte(e.xml)))
		if err != nil {
			t.Log("Error", err)
		}

		// create a test response recorder, which satisfies the requirements
		// for a ResponseWriter.
		rr := httptest.NewRecorder()

		// call ReadXML and check for an error.
		var note struct {
			To   string `xml:"to"`
			From string `xml:"from"`
		}

		err = tools.ReadXML(rr, req, &note)
		if e.errorExpected && err == nil {
			t.Errorf("%s: expected an error, but did not get one", e.name)
		} else if !e.errorExpected && err != nil {
			t.Errorf("%s: did not expect an error, but got one: %s", e.name, err)
		}
	}
}

func TestTools_ErrorXML(t *testing.T) {
	var testTools Tools

	rr := httptest.NewRecorder()
	err := testTools.ErrorXML(rr, errors.New("some error"), http.StatusServiceUnavailable)
	if err != nil {
		t.Error(err)
	}

	var requestPayload XMLResponse
	decoder := xml.NewDecoder(rr.Body)
	err = decoder.Decode(&requestPayload)
	if err != nil {
		t.Error("received error when decoding ErrorXML payload:", err)
	}

	if !requestPayload.Error {
		t.Error("error set to false in response from ErrorXML, and should be set to true")
	}

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("wrong status code returned; expected 503, but got %d", rr.Code)
	}
}
