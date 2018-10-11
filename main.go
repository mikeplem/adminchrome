package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/satori/go.uuid"
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
	TV []tvconfig
}

var showTVs = []tvconfig{}
var tvDropDown = userTVs{TV: showTVs}

// ======================

const loginTpl = `
<!DOCTYPE html>
<html>
<head>
	<title>Browser URL Control</title>
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
		<input type="text" name="url" placeholder="https://oliveai.com">
		<p />
		<input type="submit" value="Send URL">
	</form>
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
}

func login(res http.ResponseWriter, req *http.Request) {

	var userAllowedTVs []string

	formUsername := req.FormValue("username")
	formPassword := req.FormValue("password")

	log.Printf("Logging in with user: %s\n", formUsername)

	authenticated, userAllowedTVs := LDAPAuthUser(formUsername, formPassword)

	if authenticated {

		for tvKey, tvValue := range Config.TV {
			for _, authValue := range userAllowedTVs {
				authValue = strings.Replace(authValue, ",", "", -1)
				if (tvKey == authValue) || (authValue == "hqtvall") {
					tvEntry := tvconfig{Name: tvValue.Name, Host: tvValue.Host}
					tvDropDown.AddItem(tvEntry)
				}
			}
		}

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
			res.WriteHeader(http.StatusUnauthorized)
			return
		}
		res.WriteHeader(http.StatusBadRequest)
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
			res.WriteHeader(http.StatusUnauthorized)
			return
		}
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	//sessionToken := c.Value
	//log.Printf("sendURL: sessionToken: %s\n", sessionToken)

	tv := req.FormValue("tv")
	urlToBrowser := req.FormValue("url")

	log.Println(urlToBrowser)

	postData := url.Values{}
	postData.Set("u", urlToBrowser)

	tvToSend := fmt.Sprintf("http://%s:%d/open", tv, Config.Remote.Port)

	log.Printf("Sending the following: %v, %v\n", tvToSend, postData)

	resp, err := http.PostForm(tvToSend, postData)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	http.Redirect(res, req, "/tv", 302)

}

func init() {

	ConfigFile = flag.String("conf", "", "Config file for this listener and ldap configs")

	flag.Parse()

	if _, err := toml.DecodeFile(*ConfigFile, &Config); err != nil {
		log.Fatal(err)
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
