/*
Copyright 2013 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"code.google.com/p/goauth2/oauth"
)

const (
	visitMessage  = "Visit the URL below to authenticate this program:"
	openedMessage = "Your browser has been opened to an authorization URL:"
	resumeMessage = "This program will resume once authenticated."
	closeMessage  = "You may now close this browser window."
)

func authenticate(transport *oauth.Transport) error {
	code := make(chan string)

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return err
	}
	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, closeMessage)
		code <- r.FormValue("code") // send code to OAuth flow
		listener.Close()            // shut down HTTP server
	}))

	transport.Config.RedirectURL = fmt.Sprintf("http://%s/", listener.Addr())
	url := transport.Config.AuthCodeURL("")
	if err := openURL(url); err != nil {
		fmt.Fprintln(os.Stderr, visitMessage)
	} else {
		fmt.Fprintln(os.Stderr, openedMessage)
	}
	fmt.Fprintf(os.Stderr, "\n%s\n\n", url)
	fmt.Fprintln(os.Stderr, resumeMessage)

	_, err = transport.Exchange(<-code)
	return err
}

func openURL(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("Cannot open URL %s on this platform", url)
	}
	return err

}
