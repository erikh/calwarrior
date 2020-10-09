package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/oauth2"
)

const (
	credentialFile = "credential.json"
	tokenFile      = "token.json"
)

// CredentialJSON is the json version of the google credentials.
var CredentialJSON []byte

func init() {
	// take the creds from the environment
	// otherwise, credentials.json in the calwarrior dir
	// finally, take the default credentials.
	creds := os.Getenv("CALWARRIOR_CREDENTIALS")
	if creds == "" {
		dir, err := findSettingsDir()
		if err != nil {
			panic(fmt.Sprintf("Creating settings directory: %v", err))
		}

		tmp := filepath.Join(dir, credentialFile)

		if _, err := os.Stat(tmp); errors.Is(err, os.ErrNotExist) {
			CredentialJSON = []byte(`{"installed":{"client_id":"99874128152-hva5l5m298s29nhgcn3g93b1mer2idrr.apps.googleusercontent.com","project_id":"calwarrior-1601022953055","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","auth_provider_x509_cert_url":"https://www.googleapis.com/oauth2/v1/certs","client_secret":"MjS0X-K48QnTJZc65UQsGxuV","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`)
		} else {
			creds = tmp
		}
	}

	if creds != "" {
		b, err := ioutil.ReadFile(creds)
		if err != nil {
			panic(fmt.Sprintf("Cannot use custom credentials: %v", err))
		}

		CredentialJSON = bytes.TrimSpace(b)
	}
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) (*http.Client, error) {
	dir, err := findSettingsDir()
	if err != nil {
		return nil, err
	}

	tokFile := filepath.Join(dir, tokenFile)
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok), nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	tool, useTool := findLauncher()

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	if useTool {
		fmt.Println("The browser will now open; accept the prompts and type the authorization code:")

		if err := exec.Command(tool, authURL).Run(); err != nil {
			fmt.Printf("Error launching detected URL launcher: %v -- continuing in manual mode\n", tool)
			useTool = false
		}
	}

	if !useTool {
		fmt.Printf("Go to the following link in your browser then type the "+
			"authorization code: \n%v\n", authURL)
	}

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
