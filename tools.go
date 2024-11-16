package Toolbox_Reusable_Go_module

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// randomStringSource is the source for generating random strings.
const randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0987654321_+"

// defaultMaxUpload is the default max upload size (10 mb)
const defaultMaxUpload = 10485760

// Tools is the type for this package. Create a variable of this type, and you have access
// to all the exported methods with the receiver type *Tools.
type Tools struct {
	MaxJSONSize        int         // maximum size of JSON file we'll process
	MaxXMLSize         int         // maximum size of XML file we'll process
	MaxFileSize        int         // maximum size of uploaded files in bytes
	AllowedFileTypes   []string    // allowed file types for upload (e.g. image/jpeg)
	AllowUnknownFields bool        // if set to true, allow unknown fields in JSON
	ErrorLog           *log.Logger // the info log.
	InfoLog            *log.Logger // the error log.
}

// New returns a new toolbox with sensible defaults.
func New() Tools {
	return Tools{
		MaxJSONSize: defaultMaxUpload,
		MaxXMLSize:  defaultMaxUpload,
		MaxFileSize: defaultMaxUpload,
		InfoLog:     log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime),
		ErrorLog:    log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

// JSONResponse is the type used for sending JSON around.
type JSONResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// XMLResponse is the type used for sending XML around.
type XMLResponse struct {
	Error   bool        `xml:"error"`
	Message string      `xml:"message"`
	Data    interface{} `xml:"data,omitempty"`
}

// ReadJSON tries to read the body of a request and converts it from JSON to a variable. The third parameter, data,
// is expected to be a pointer, so that we can read data into it.
func (t *Tools) ReadJSON(w http.ResponseWriter, r *http.Request, data interface{}) error {

	// Check content-type header; it should be application/json. If it's not specified,
	// try to decode the body anyway.
	if r.Header.Get("Content-Type") != "" {
		contentType := r.Header.Get("Content-Type")
		if strings.ToLower(contentType) != "application/json" {
			return errors.New("the Content-Type header is not application/json")
		}
	}

	// Set a sensible default for the maximum payload size.
	maxBytes := defaultMaxUpload

	// If MaxJSONSize is set, use that value instead of default.
	if t.MaxJSONSize != 0 {
		maxBytes = t.MaxJSONSize
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)

	// Should we allow unknown fields?
	if !t.AllowUnknownFields {
		dec.DisallowUnknownFields()
	}

	// Attempt to decode the data, and figure out what the error is, if any, to send back a human-readable
	// response.
	err := dec.Decode(data)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			return fmt.Errorf("body contains incorrect JSON type for field %q at offset %d", unmarshalTypeError.Field, unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)

		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)

		case errors.As(err, &invalidUnmarshalError):
			return fmt.Errorf("error unmarshalling json: %s", err.Error())

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

// WriteJSON takes a response status code and arbitrary data and writes a JSON response to the client.
func (t *Tools) WriteJSON(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// If we have a value as the last parameter in the function call, then we are setting a custom header.
	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	// Set the content type and send response.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(out)

	return nil
}

// ErrorJSON takes an error, and optionally a response status code, and generates and sends
// a JSON error response.
func (t *Tools) ErrorJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest

	// If a custom response code is specified, use that instead of bad request.
	if len(status) > 0 {
		statusCode = status[0]
	}

	// Build the JSON payload.
	var payload JSONResponse
	payload.Error = true
	payload.Message = err.Error()

	return t.WriteJSON(w, statusCode, payload)
}

// RandomString returns a random string of letters of length n, using characters specified in randomStringSource.
func (t *Tools) RandomString(n int) string {
	s, r := make([]rune, n), []rune(randomStringSource)
	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}
	return string(s)
}

// PushJSONToRemote posts arbitrary json to some url, and returns the response, the response
// status code, and error, if any. The final parameter, client, is optional, and will default
// to the standard http.Client. It exists to make testing possible without an active remote
// url.
func (t *Tools) PushJSONToRemote(uri string, data interface{}, client ...*http.Client) (*http.Response, int, error) {
	// create json we'll send
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, 0, err
	}

	httpClient := &http.Client{}
	if len(client) > 0 {
		httpClient = client[0]
	}

	// Build the request and set header.
	request, err := http.NewRequest("POST", uri, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Content-Type", "application/json")

	// Call the url.
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()

	return response, response.StatusCode, nil
}

// DownloadStaticFile downloads a file to the remote user, and tries to force the browser to avoid displaying it in
// the browser window by setting content-disposition. It also allows specification of the display name.
func (t *Tools) DownloadStaticFile(w http.ResponseWriter, r *http.Request, p, file, displayName string) {
	fp := path.Join(p, file)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", displayName))

	http.ServeFile(w, r, fp)
}

// UploadedFile is the type used for the uploaded file.
type UploadedFile struct {
	NewFileName      string
	OriginalFileName string
	FileSize         int64
}

// UploadOneFile is just a convenience method that calls UploadFiles, but expects only one file to
// be in the upload.
func (t *Tools) UploadOneFile(r *http.Request, uploadDir string, rename ...bool) (*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	files, err := t.UploadFiles(r, uploadDir, renameFile)
	if err != nil {
		return nil, err
	}

	return files[0], nil
}

// UploadFiles uploads one or more file to a specified directory, and gives the files a random name.
// It returns a slice containing the newly named files, the original file names, the size of the files,
// and potentially an error. If the optional last parameter is set to true, then we will not rename
// the files, but will use the original file names.
func (t *Tools) UploadFiles(r *http.Request, uploadDir string, rename ...bool) ([]*UploadedFile, error) {
	// check to see if we are renaming the uploadedFiles with the optional last parameter.
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	var uploadedFiles []*UploadedFile

	// Create the upload directory if it does not exist.
	err := t.CreateDirIfNotExist(uploadDir)
	if err != nil {
		return nil, err
	}

	// Sanity check on t.MaxFileSize.
	if t.MaxFileSize == 0 {
		t.MaxFileSize = defaultMaxUpload
	}

	// Parse the form, so we have access to the file.
	err = r.ParseMultipartForm(int64(t.MaxFileSize))
	if err != nil {
		return nil, fmt.Errorf("error parsing form data: %v", err)
	}

	for _, fHeaders := range r.MultipartForm.File {
		for _, hdr := range fHeaders {
			uploadedFiles, err = func(uploadedFiles []*UploadedFile) ([]*UploadedFile, error) {
				var uploadedFile UploadedFile
				infile, err := hdr.Open()
				if err != nil {
					return nil, err
				}
				defer infile.Close()

				if hdr.Size > int64(t.MaxFileSize) {
					return nil, fmt.Errorf("the uploaded file is too big, and must be less than %d", t.MaxFileSize)
				}

				buff := make([]byte, 512)
				_, err = infile.Read(buff)
				if err != nil {
					return nil, err
				}

				allowed := false
				filetype := http.DetectContentType(buff)
				if len(t.AllowedFileTypes) > 0 {
					for _, x := range t.AllowedFileTypes {
						if strings.EqualFold(filetype, x) {
							allowed = true
						}
					}
				} else {
					allowed = true
				}

				if !allowed {
					return nil, errors.New("the uploaded file type is not permitted")
				}

				_, err = infile.Seek(0, 0)
				if err != nil {
					fmt.Println(err)
					return nil, err
				}

				if renameFile {
					uploadedFile.NewFileName = fmt.Sprintf("%s%s", t.RandomString(25), filepath.Ext(hdr.Filename))
				} else {
					uploadedFile.NewFileName = hdr.Filename
				}
				uploadedFile.OriginalFileName = hdr.Filename

				var outfile *os.File
				defer outfile.Close()

				if outfile, err = os.Create(filepath.Join(uploadDir, uploadedFile.NewFileName)); nil != err {
					return nil, err
				}
				fileSize, err := io.Copy(outfile, infile)
				if err != nil {
					return nil, err
				}
				uploadedFile.FileSize = fileSize

				uploadedFiles = append(uploadedFiles, &uploadedFile)

				return uploadedFiles, nil
			}(uploadedFiles)
			if err != nil {
				return uploadedFiles, err
			}
		}
	}
	return uploadedFiles, nil
}

// CreateDirIfNotExist creates a directory, and all necessary parent directories, if it does not exist.
func (t *Tools) CreateDirIfNotExist(path string) error {
	const mode = 0755
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, mode)
		if err != nil {
			return err
		}
	}
	return nil
}

// Slugify is a (very) simple means of creating a slug from a provided string.
func (t *Tools) Slugify(s string) (string, error) {
	if s == "" {
		return "", errors.New("empty string not permitted")
	}
	var re = regexp.MustCompile(`[^a-z\d]+`)
	slug := strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(slug) == 0 {
		return "", errors.New("after removing characters, slug is zero length")
	}

	return slug, nil
}

// WriteXML takes a response status code and arbitrary data and writes an XML response to the client.
// The Content-Type header is set to application/xml.
func (t *Tools) WriteXML(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	out, err := xml.Marshal(data)
	if err != nil {
		return err
	}

	// If we have a value as the last parameter in the function call, then we are setting a custom header.
	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	// Set the content type and send response. According to RFC 7303, text/xml and application/xml are to be
	// treated as the same, so we'll just pick one.
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)

	// Add the XML header.
	xmlOut := []byte(xml.Header + string(out))
	_, _ = w.Write(xmlOut)

	return nil
}

// ReadXML tries to read the body of an XML request into a variable. The third parameter, data,
// is expected to be a pointer, so that we can read data into it.
func (t *Tools) ReadXML(w http.ResponseWriter, r *http.Request, data interface{}) error {
	maxBytes := defaultMaxUpload

	// If MaxXMLSize is set, use that value instead of default.
	if t.MaxXMLSize != 0 {
		maxBytes = t.MaxXMLSize
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := xml.NewDecoder(r.Body)

	// Attempt to decode the data.
	err := dec.Decode(data)
	if err != nil {
		return err
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single XML value")
	}

	return nil
}

// ErrorXML takes an error, and optionally a response status code, and generates and sends
// an XML error response.
func (t *Tools) ErrorXML(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest

	// If a custom response code is specified, use that instead of bad request.
	if len(status) > 0 {
		statusCode = status[0]
	}

	var payload XMLResponse
	payload.Error = true
	payload.Message = err.Error()

	return t.WriteXML(w, statusCode, payload)
}
