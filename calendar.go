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
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

// CredentialJSON is the json version of the google credentials.
// Please note that the trim just makes it a touch more readable.
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

		tmp := filepath.Join(dir, "credential.json")

		if _, err := os.Stat(tmp); errors.Is(err, os.ErrNotExist) {
			CredentialJSON = bytes.TrimSpace([]byte(`
{"installed":{"client_id":"99874128152-hva5l5m298s29nhgcn3g93b1mer2idrr.apps.googleusercontent.com","project_id":"calwarrior-1601022953055","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","auth_provider_x509_cert_url":"https://www.googleapis.com/oauth2/v1/certs","client_secret":"MjS0X-K48QnTJZc65UQsGxuV","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}
`))
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

func toCalendarTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

func gatherEvents(srv *calendar.Service, t1, t2 time.Time) (*calendar.Events, error) {
	return srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(toCalendarTime(t1)).TimeMax(toCalendarTime(t2)).OrderBy("startTime").Do()
}

func insertEvent(srv *calendar.Service, event *calendar.Event) (*calendar.Event, error) {
	return srv.Events.Insert("primary", event).Do()
}

func modifyEvent(srv *calendar.Service, event *calendar.Event) (*calendar.Event, error) {
	return srv.Events.Patch("primary", event.Id, event).Do()
}

func getEvent(srv *calendar.Service, calID string) (*calendar.Event, error) {
	return srv.Events.Get("primary", calID).Do()
}

func deleteEvent(srv *calendar.Service, calID string) error {
	return srv.Events.Delete("primary", calID).Do()
}

func getCalendarClient() (*calendar.Service, error) {
	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(CredentialJSON, calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse client secret file to config: %w", err)
	}

	client, err := getClient(config)
	if err != nil {
		return nil, fmt.Errorf("Trouble while gathering client information: %w", err)
	}

	srv, err := calendar.New(client)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve Calendar client: %w", err)
	}

	return srv, nil
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) (*http.Client, error) {
	dir, err := findSettingsDir()
	if err != nil {
		return nil, err
	}

	tokFile := filepath.Join(dir, "token.json")
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok), nil
}

func findSettingsDir() (string, error) {
	osSettingsMap := map[string][]string{
		"linux":  []string{os.Getenv("XDG_CONFIG_HOME"), filepath.Join(os.Getenv("HOME"), ".config")},
		"darwin": []string{filepath.Join(os.Getenv("HOME"), "Library")},
	}

	for _, dir := range osSettingsMap[runtime.GOOS] {
		if dir != "" {
			dir = filepath.Join(dir, "calwarrior")

			// not really sure where best to do this.
			if err := os.MkdirAll(dir, 0700); err != nil {
				return "", err
			}
			return dir, nil
		}
	}

	return "", errors.New("cannot find the appropriate dir to set configuration; try setting $HOME, container nerd")
}

func findLauncher() (string, bool) {
	osToolMap := map[string]string{
		"linux":  "xdg-open",
		"darwin": "open",
	}

	tool := osToolMap[runtime.GOOS]
	if s, err := exec.LookPath(tool); err == nil {
		return s, true
	}

	return "", false
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
