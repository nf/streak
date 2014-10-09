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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/golang/oauth2"
	"github.com/golang/oauth2/google"
)

const (
	visitMessage  = "Visit the URL below to authenticate this program:"
	openedMessage = "Your browser has been opened to an authorization URL:"
	resumeMessage = "This program will resume once authenticated."
	closeMessage  = "You may now close this browser window."
)

func oauthConfig(redirectURL string) (*oauth2.Config, error) {
	return google.NewConfig(&oauth2.Options{
		ClientID:     "120233572441-d8vmojicfgje467joivr5a7j52dg2gnc.apps.googleusercontent.com",
		ClientSecret: "vfZkluBV6PTfGBWxfIIyXbMS",
		RedirectURL:  redirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/calendar"},
	})
}

func oauthTransport(existing *oauth2.Token) (*oauth2.Transport, error) {
	if existing != nil {
		cfg, err := oauthConfig("http://example.org/ignored")
		if err != nil {
			return nil, err
		}
		t := cfg.NewTransport()
		t.SetToken(existing)
		return t, nil
	}

	code := make(chan string)

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}
	defer l.Close()
	go http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, closeMessage)
		code <- r.FormValue("code") // send code to OAuth flow
	}))

	cfg, err := oauthConfig(fmt.Sprintf("http://%s/", l.Addr().String()))
	if err != nil {
		return nil, err
	}
	url := cfg.AuthCodeURL("", "online", "auto")
	if err := openURL(url); err != nil {
		fmt.Fprintln(os.Stderr, visitMessage)
	} else {
		fmt.Fprintln(os.Stderr, openedMessage)
	}
	fmt.Fprintf(os.Stderr, "\n%s\n\n", url)
	fmt.Fprintln(os.Stderr, resumeMessage)

	return cfg.NewTransportWithCode(<-code)
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

func readToken(file string) (*oauth2.Token, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(b, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func writeToken(file string, tok *oauth2.Token) error {
	b, err := json.Marshal(tok)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, b, 0600)
}
