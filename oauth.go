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
	"log"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	//"github.com/golang/oauth2"
	//"github.com/golang/oauth2/google"
)

const (
	visitMessage  = "Visit the URL below to authenticate this program:"
	openedMessage = "Your browser has been opened to an authorization URL:"
	resumeMessage = "This program will resume once authenticated."
	closeMessage  = "You may now close this browser window."
)

func oauthConfig(redirectURL string) (*oauth2.Config ) {
	//return google.NewConfig(&oauth2.Options{
	return &oauth2.Config{
		ClientID:     "49750719428-mgkorp1ad5h0e4ug4h5vq0psavl0u9it.apps.googleusercontent.com",
		ClientSecret: "EPPJVMBAjQp3Es3GquymG2Mb",
		RedirectURL:  redirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/calendar"},
		Endpoint:  google.Endpoint,
	}
}

//func oauthTransport(existing *oauth2.Token) (*oauth2.Transport, error) {
func oauthClientToken(existing *oauth2.Token) (*http.Client, *oauth2.Token) {
	if existing != nil {
		cfg := oauthConfig("localhost")
		//if err != nil {
		//	return nil, err
		//}
		//t := cfg.NewTransport()
		//t.SetToken(existing)
		//return t, nil
		client := cfg.Client(oauth2.NoContext, existing)
		return client, existing
	}

	ch := make(chan string)
	randState := fmt.Sprintf("st%d", time.Now().UnixNano())

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, nil //, err
	}
	defer l.Close()
	go http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			http.Error(w, "", 404)
			return
		}
		if r.FormValue("state") != randState {
			log.Printf("State doesn't match: req = %#v", r)
			http.Error(w, "", 500)
			return
		}
		//fmt.Fprint(w, closeMessage)
		//code <- r.FormValue("code") // send code to OAuth flow
		if code := r.FormValue("code"); code != "" {
			fmt.Fprintf(w, "<h1>Success</h1>Authorized.")
			w.(http.Flusher).Flush()
			ch <- code
			return
		}
	}))

	cfg := oauthConfig(fmt.Sprintf("http://%s/", l.Addr().String()))
	if err != nil {
		return nil, nil //, err
	}
	url := cfg.AuthCodeURL(randState, oauth2.AccessTypeOffline)
	if err := openURL(url); err != nil {
		fmt.Fprintln(os.Stderr, visitMessage)
	} else {
		fmt.Fprintln(os.Stderr, openedMessage)
	}
	fmt.Fprintf(os.Stderr, "\n%s\n\n", url)
	fmt.Fprintln(os.Stderr, resumeMessage)

	//return cfg.NewTransportWithCode(<-code)
	//var code string
	//if _, err := fmt.Scan(&code); err != nil {
	//	log.Fatal(err)
	//}
	tok, err := cfg.Exchange(oauth2.NoContext, <-ch)
	if err != nil {
		fmt.Println("Fatal error")
		log.Fatal(err)
	}
	client := cfg.Client(oauth2.NoContext, tok)
	return client, tok
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
