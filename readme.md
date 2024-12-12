[![Version](https://img.shields.io/badge/goversion-1.19.x-blue.svg)](https://golang.org)
<a href="https://golang.org"><img src="https://img.shields.io/badge/powered_by-Go-3362c2.svg?style=flat-square" alt="Built with GoLang"></a>
[![License](http://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://raw.githubusercontent.com/tsawler/toolbox/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/tsawler/toolbox)](https://goreportcard.com/report/github.com/tsawler/toolbox)
![Tests](https://github.com/tsawler/toolbox/actions/workflows/tests.yml/badge.svg)
<a href="https://pkg.go.dev/github.com/tsawler/toolbox"><img src="https://img.shields.io/badge/godoc-reference-%23007d9c.svg"></a>
[![Go Coverage](https://github.com/tsawler/toolbox/wiki/coverage.svg)](https://raw.githack.com/wiki/tsawler/toolbox/coverage.html)

# Toolbox 

A simple example of how to create a reusable Go module with commonly used tools.  

The included tools are: 

- Read JSON
- Write JSON
- Produce a JSON encoded error response
- Write XML
- Read XML
- Produce an XML encoded error response
- Upload a file to a specified directory
- Download a static file
- Get a random string of length n
- Post JSON to a remote service 
- Create a directory, including all parent directories, if it does not already exist
- Create a URL safe slug from a string

## Installation

`go get -u github.com/tsawler/toolbox`


## Usage

```go
package main

import (
	"fmt"
	"github.com/tsawler/toolbox"
)

func main() {
	// create a variable of type toolbox.Tools, so we can use this variable
	// to call the methods on that type
	var tools toolbox.Tools

	// get a random string
	rnd := tools.RandomString(10)
	fmt.Println(rnd)
}
```

### Working with JSON

In a handler, for example:

```go
// JSONPayload is the type for JSON data that we receive
type JSONPayload struct {
    Name string `json:"name"`
    Data string `json:"data"`
}

// SomeHandler is the handler to accept a post request consisting of json payload
func (app *Config) SomeHandler(w http.ResponseWriter, r *http.Request) {
    var tools toolbox.Tools
    
    // read json into var
    var requestPayload JSONPayload
    _ = tools.ReadJSON(w, r, &requestPayload)
	
    // do something with the data here...
    
    // create the response we'll send back as JSON
    resp := toolbox.JSONResponse{
        Error:   false,
        Message: "logged",
    }
    
    // write the response back as JSON
    _ = tools.WriteJSON(w, http.StatusAccepted, resp)
}
```

### Download a file

To download a static file, and force it to download instead of displaying
in a browser:

```go
// DownloadAFile downloads an arbitrary file
func (app *Config) DownloadAFile(w http.ResponseWriter, r *http.Request) {
    var tools toolbox.Tools

    tools.DownloadStaticFile(w, r, "./data", "file.pdf", "file.pdf")
}
```

### Creating a directory

To create a directory if it does not already exist:

```go
// SomeHandler is some kind of handler
func (app *Config) SomeHandler(w http.ResponseWriter, r *http.Request) {
    var tools toolbox.Tools

    err := tools.CreateDirIfNotExist("./myDir")
    if err != nil {
        // do something with the error...
    }
	
    // keep going in the handler...
}
```

### Uploading a File:

To upload a file to a specific directory, with this for HTML:

```html
<!doctype html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport"
          content="width=device-width, user-scalable=no, initial-scale=1.0, maximum-scale=1.0, minimum-scale=1.0">
    <meta http-equiv="X-UA-Compatible" content="ie=edge">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.2.0-beta1/dist/css/bootstrap.min.css" rel="stylesheet"
          integrity="sha384-0evHe/X+R7YkIZDRvuzKMRqM+OrBnVFBL6DOitfPri4tjfHxaWutUpFmBp4vmVor" crossorigin="anonymous">
    <title>Upload test</title>
</head>
<body>
<div class="container">
    <div class="row">
        <div class="col">
            <h1 class="mt-2">Upload a file</h1>
            <hr>

            <form action="http://localhost:8080/upload" method="post" enctype="multipart/form-data">

                <div class="mb-3">
                    <label for="fileUpload" class="form-label">Choose file(s) to upload...</label>
                    <input class="form-control" type="file" id="fileUpload" name="uploaded" multiple>
                </div>


                <input class="btn btn-primary" type="submit" value="Upload file">
            </form>

        </div>
    </div>
</div>
</body>
</html>
```
And this for a Go application:

```go
package main

import (
	"fmt"
	"github.com/tsawler/toolbox"
	"log"
	"net/http"
)

func main() {

	// handle html route (http://localhost:8080/)
	http.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir("."))))

	// Post handler 
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		t := toolbox.Tools{
			MaxFileSize:      1024 * 1024 * 1024,
			AllowedFileTypes: []string{"image/gif", "image/png", "image/jpeg"},
		}
		

		// Upload the file(s). Note that if you don't want the files to be renamed,
		// you can add an optional final parameter -- true will rename the files (the default)
		// and false will preserve the original filenames, for example:
		// files, err := t.UploadFiles(r, "./uploads", false)
		// n.b.: if the "./uploads" directory does not exist, we attempt to create it.
		files, err := t.UploadFiles(r, "./uploads")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// the returned variable, files, will be a slice of the type toolbox.UploadedFile
		_, _ = w.Write([]byte(fmt.Sprintf("Uploaded %d file(s) to the uploads folder", len(files))))	
	})

	// print a log message
	log.Println("Starting server on port 8080")

	// start the server
	http.ListenAndServe(":8080", nil)
}
```

### Calling a Remote API

To make a JSON post to a remote URI, with this html:

```html
<!doctype html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport"
          content="width=device-width, user-scalable=no, initial-scale=1.0, maximum-scale=1.0, minimum-scale=1.0">
    <meta http-equiv="X-UA-Compatible" content="ie=edge">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css" rel="stylesheet"
          integrity="sha384-1BmE4kWBq78iYhFldvKuhfTAU6auU8tT94WrHftjDbrCEXSU1oBoqyl2QvZ6jIW3"
          crossorigin="anonymous">

    <title>JSON functionality</title>
    <style>
        label{
            font-weight: bold;
        }
    </style>
</head>
<body>
<div class="container">
    <div class="row">
        <div class="col">
            <h1 class="mt-2">JSON functionality</h1>
            <hr>

            <form>


                <div class="mb-3">
                    <label for="json" class="form-label">JSON to Send:</label>
                    <textarea style="font-family: Courier,sans-serif" class="form-control"
                              id="json" name="json" rows="5">
{
    "action": "some action",
    "message": "some message"
}
                    </textarea>
                </div>


                <a id="pushBtn" class="btn btn-primary">Push JSON</a>
            </form>
            <hr>
            <p><strong>Response from server:</strong></p>
            <div style="outline: 1px solid silver; padding: 2em">
                <pre id="response">No response from server yet...</pre>
            </div>

        </div>
    </div>
</div>

<script>
    let pushBtn = document.getElementById("pushBtn");
    let jsonPayload = document.getElementById("json")
    let serverResponse = document.getElementById("response");

    pushBtn.addEventListener("click", function () {
        const payload = jsonPayload.value;
        const headers = new Headers();

        const body = {
            method: 'POST',
            body: payload,
            headers: headers,
        }

        headers.append("Content-Type", "application/json");

        fetch("http://localhost:8081/receive-post", body)
            .then((response) => response.json())
            .then((data) => {
                serverResponse.innerHTML = JSON.stringify(data, undefined, 4);
            })
            .catch((error) => {
                serverResponse.innerHTML = "<br><br>Error: " + error;
            })
    })
</script>
</body>
</html>
```

You can use this kind of Go code:

```go
package main

import (
	"github.com/tsawler/toolbox"
	"log"
	"net/http"
)

func main() {
	// create a default server mux
	mux := http.NewServeMux()

	// register routes
	mux.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir("."))))
	mux.HandleFunc("/receive-post", receivePost)
	mux.HandleFunc("/remote-service", remoteService)

	// print a log message
	log.Println("Starting server on port 8081")

	// start the server
	err := http.ListenAndServe(":8081", mux)
	if err != nil {
		log.Fatal(err)
	}
}

// RequestPayload describes the JSON that this service accepts as an HTTP Post request
type RequestPayload struct {
	Action  string `json:"action"`
	Message string `json:"message"`
}

// ResponsePayload is the structure used for sending a JSON response
type ResponsePayload struct {
	Message    string `json:"message"`
	StatusCode int    `json:"status_code,omitempty"`
}

func receivePost(w http.ResponseWriter, r *http.Request) {
	// get the posted json and decode it
	var requestPayload RequestPayload
	var t toolbox.Tools

	err := t.ReadJSON(w, r, &requestPayload)
	if err != nil {
		_ = t.ErrorJSON(w, err)
		return
	}

	// Call remote service. Note that we are ignoring the first return parameter, which is the 
	// entire response from the remote service, but you have access to it if you need it.
	_, statusCode, err := t.PushJSONToRemote("http://localhost:8081/remote-service", requestPayload)
	if err != nil {
		_ = t.ErrorJSON(w, err)
		return
	}

	// send response
	payload := ResponsePayload{
		Message:    "hit the service ok",
		StatusCode: statusCode,
	}

	err = t.WriteJSON(w, http.StatusAccepted, payload)
	if err != nil {
		log.Println(err)
	}
}

// remoteService just simulates calling some remote API
func remoteService(w http.ResponseWriter, r *http.Request) {
	payload := ResponsePayload{
		Message: "OK",
	}
	var t toolbox.Tools

	_ = t.WriteJSON(w, http.StatusOK, payload)
}
```

### Create a slug from a string

To slugify a string, we simply remove all non URL safe characters and return the
original string with a hyphen where spaces would be. Example:

```go
package main

import (
	"fmt"
	"github.com/tsawler/toolbox"
)

func main() {
	toSlugify := "hello, world! These are unsafe chars: こんにちは世界*!&^%"
	fmt.Println("To slugify:", toSlugify)
	var tools toolbox.Tools

	slug, err := tools.Slugify(toSlugify)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Slugified:", slug)
}
```

Output from this is:

```
To slugify: hello, world! These are unsafe chars: こんにちは世界*!&^%
Slugified: hello-world-these-are-unsafe-chars
```
