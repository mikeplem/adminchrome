package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	uuid "github.com/satori/go.uuid"
)

// ======================

// ConfigFile holds the user supplied configuration file - it is placed here since it is a global
var ConfigFile *string

// Config is the structure of the TOML config structure
var Config tomlConfig

type tomlConfig struct {
	Listen listenconfig `toml:"listen"`
	LDAP   ldapconfig   `toml:"ldap"`
	Remote remoteconfig `toml:"remote"`
	TV     map[string]tvconfig
}

type listenconfig struct {
	SSL  bool
	Cert string
	Key  string
	Port int
}

type remoteconfig struct {
	Port int
}

type ldapconfig struct {
	UseLDAP      bool
	Host         string
	Port         int
	Base         string
	GroupBase    string
	BindDN       string
	BindPassword string
}

type tvconfig struct {
	Name string
	Host string
}

// struct and vars used to hold TVs allowed by the user
type userTVs struct {
	TV tvConfigs
}

type tvConfigs []tvconfig

func (t tvConfigs) Len() int           { return len(t) }
func (t tvConfigs) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t tvConfigs) Less(i, j int) bool { return t[i].Name < t[j].Name }

var showTVs = tvConfigs{}
var tvDropDown = userTVs{TV: showTVs}

// ======================

const loginTpl = `
<!DOCTYPE html>
<html>
<head>
	<title>Browser URL Control</title>

	<style>
	body {
		background-color: #d3d4d5;
		font-size: 16pt;
	}
	</style>
</head>


<body>
	<h1>Browser URL Control</h1>
	Login with your LDAP credentials. 
	<p />
	<form method="POST" action="/login">
		Please fill in your LDAP username
		<input type="text" name="username" placeholder="username">
		<br>
		Please fill in your LDAP password
		<input type="password" name="password" placeholder="password">
		<p />
		<input type="submit" value="Login">
	</form>
</body>
</html>`

const tvTpl = `
<!DOCTYPE html>
<html>
<head>
	<title>Send to TV</title>
	<style>
	body {
		background-color: #d3d4d5;
		font-size: 16pt;
	}
	</style>

</head>

<body>
	<form method="POST" action="/sendurl">
		Please select the TV you want to send the URL
		<select name="tv">
		{{ range .TV -}}
		<option value="{{.Host}}">{{.Name}}</option> 
		{{ end }}
		</select>
		<p />
		Please fill in the URL you want to send
		<input type="text" name="url" size="100" placeholder="https://oliveai.com">
		<p />
		<div>
		<input type="checkbox" id="reload" name="reload">
		<label for="reload">Reload Browser</label>
		</div>
		<p />
		<input type="submit" value="Send URL">
	</form>
</body>
</html>`

const errorTpl = `
<!DOCTYPE html>
<html>
<head>
	<title>Error</title>

	<style>
	body {
		background-color: #d3d4d5;
		font-size: 16pt;
	}
	</style>
</head>


<body>
	<h1>There was an error</h1>
	{{ . }} 
	<p />
	<a href='/'>Please click here to go back to the main page</a>
</body>
</html>`

// ========================

// method used to add user allowed TVs to struct for use
// with the html dropdown template for sending a URL to
// a TV
func (t *userTVs) AddItem(item tvconfig) []tvconfig {
	t.TV = append(t.TV, item)
	return t.TV
}

func homePage(res http.ResponseWriter, req *http.Request) {

	if Config.LDAP.UseLDAP {

		t, err := template.New("webpage").Parse(loginTpl)
		if err != nil {
			log.Print(err)
			return
		}

		err = t.Execute(res, Config)
		if err != nil {
			log.Print("execute: ", err)
			return
		}

	} else {

		// create session token for user
		sessionToken, err := uuid.NewV4()
		if err != nil {
			log.Printf("sessionToken failed to create: %s", err)
			return
		}

		// Finally, we set the client cookie for "session_token" as the session token we just generated
		// we also set an expiry time of 120 seconds
		http.SetCookie(res, &http.Cookie{
			Name:    "session_token",
			Value:   sessionToken.String(),
			Expires: time.Now().Add(120 * time.Second),
		})

		log.Println("/ redirect to /tv")
		http.Redirect(res, req, "/tv", 302)

	}
}

func login(res http.ResponseWriter, req *http.Request) {

	var userAllowedTVs []string

	formUsername := req.FormValue("username")
	formPassword := req.FormValue("password")

	log.Printf("Logging in with user: %s\n", formUsername)

	authenticated, userAllowedTVs := LDAPAuthUser(formUsername, formPassword)

	tvDropDown = userTVs{}

	for tvKey, tvValue := range Config.TV {
		for _, authValue := range userAllowedTVs {
			authValue = strings.Replace(authValue, ",", "", -1)
			if (tvKey == authValue) || (authValue == "hqtvall") {
				tvEntry := tvconfig{Name: tvValue.Name, Host: tvValue.Host}
				tvDropDown.AddItem(tvEntry)
			}
		}
	}

	sort.Sort(tvDropDown.TV)

	if authenticated {

		// create session token for user
		sessionToken, err := uuid.NewV4()
		if err != nil {
			log.Printf("sessionToken failed to create: %s", err)
			return
		}

		// Finally, we set the client cookie for "session_token" as the session token we just generated
		// we also set an expiry time of 120 seconds
		http.SetCookie(res, &http.Cookie{
			Name:    "session_token",
			Value:   sessionToken.String(),
			Expires: time.Now().Add(120 * time.Second),
		})

		//log.Printf("login cookie set with token: %s\n", sessionToken.String())
		log.Println("/login redirect to /tv")
		http.Redirect(res, req, "/tv", 302)

	} else {
		log.Println("User did not authenticate.")
		http.Redirect(res, req, "/", 302)
	}

}

func listTVs(res http.ResponseWriter, req *http.Request) {
	_, err := req.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			log.Println("tv: cookie not set")
			errorHandler(res, req, http.StatusUnauthorized)
			return
		}
		errorHandler(res, req, http.StatusBadRequest)
		return
	}

	//sessionToken := c.Value
	//log.Printf("tv: sessionToken: %s\n", sessionToken)

	t, err := template.New("webpage").Parse(tvTpl)
	if err != nil {
		log.Print(err)
		return
	}

	err = t.Execute(res, tvDropDown)
	if err != nil {
		log.Print("execute: ", err)
		return
	}
}

func sendURL(res http.ResponseWriter, req *http.Request) {
	_, err := req.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			log.Println("sendURL: cookie not set")
			errorHandler(res, req, http.StatusUnauthorized)
			return
		}
		errorHandler(res, req, http.StatusBadRequest)
		return
	}

	//sessionToken := c.Value
	//log.Printf("sendURL: sessionToken: %s\n", sessionToken)

	tv := req.FormValue("tv")
	urlToBrowser := req.FormValue("url")
	reloadBrowser := req.FormValue("reload")

	// is the remote host and port accessible
	remoteHost := fmt.Sprintf("%s:%d", tv, Config.Remote.Port)
	conn, err := net.Dial("tcp", remoteHost)
	if err != nil {
		log.Println("Connection error:", err)
		errorHandler(res, req, http.StatusGone)
		return
	}

	defer conn.Close()

	if reloadBrowser == "on" {

		tvToReload := fmt.Sprintf("http://%s:%d/reload", tv, Config.Remote.Port)

		log.Printf("Reloading the browser: %v\n", tvToReload)

		resp, err := http.Get(tvToReload)
		if err != nil {
			errorHandler(res, req, http.StatusNotFound)
			return
		}
		defer resp.Body.Close()

		http.Redirect(res, req, "/tv", 302)

	} else {

		log.Println(urlToBrowser)

		postData := url.Values{}
		postData.Set("u", urlToBrowser)

		tvToSend := fmt.Sprintf("http://%s:%d/open", tv, Config.Remote.Port)

		log.Printf("Sending the following: %v, %v\n", tvToSend, postData)

		resp, err := http.PostForm(tvToSend, postData)
		if err != nil {
			errorHandler(res, req, http.StatusNotFound)
			return
		}
		defer resp.Body.Close()
	}
	http.Redirect(res, req, "/tv", 302)

}

func errorHandler(res http.ResponseWriter, req *http.Request, status int) {
	res.WriteHeader(status)

	var pageError = ""

	switch status {
	case http.StatusGone:
		pageError = "The remote server appears to be down.  Please contact Infrastructure that the TV appears to be down."
	case http.StatusNotFound:
		pageError = "For some reason the page could not be loaded."
	case http.StatusUnauthorized:
		pageError = "Either you are not allowed to login via an LDAP group or your session cookie has expired (2 minute lifetime)."
	case http.StatusBadRequest:
		pageError = "For some reason that command could be run."
	}

	t, err := template.New("webpage").Parse(errorTpl)
	if err != nil {
		log.Print(err)
		return
	}

	err = t.Execute(res, pageError)
	if err != nil {
		log.Print("execute: ", err)
		return
	}

}

func init() {

	ConfigFile = flag.String("conf", "", "Config file for this listener and ldap configs")

	flag.Parse()

	if _, err := toml.DecodeFile(*ConfigFile, &Config); err != nil {
		log.Fatal(err)
	}

	if !Config.LDAP.UseLDAP {
		log.Println("Authentication has been disabled by the configuration.")

		for _, tvValue := range Config.TV {
			tvEntry := tvconfig{Name: tvValue.Name, Host: tvValue.Host}
			tvDropDown.AddItem(tvEntry)
		}
	}

}

func main() {

	// Setup the proper format for the listening port on any interface
	listenPort := fmt.Sprintf(":%d", Config.Listen.Port)

	http.HandleFunc("/", homePage)
	http.HandleFunc("/login", login)
	http.HandleFunc("/tv", listTVs)
	http.HandleFunc("/sendurl", sendURL)

	if Config.Listen.SSL == true {
		log.Println("Listening on port " + listenPort + " with SSL")
		err := http.ListenAndServeTLS(listenPort, Config.Listen.Cert, Config.Listen.Key, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	} else {
		log.Println("Listening on port " + listenPort + " without SSL")
		err := http.ListenAndServe(listenPort, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}
}
