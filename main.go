package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/satori/go.uuid"
)

// ConfigFile holds the user supplied configuration file - it is placed here since it is a global
var ConfigFile *string

// Config is the structure of the TOML config structure
var Config tomlConfig

// allowedTvs holds the slice of TVs the user is allowed to access
var allowedTvs = make([]string, 0)

// validTV will contain only the TVs a user is allowed to access
var validTV tvConfig

type tomlConfig struct {
	Listen listenconfig `toml:"listen"`
	LDAP   ldapconfig   `toml:"ldap"`
	Remote remoteconfig `toml:"remote"`
	TV     map[string]tvlist
}

type tvConfig struct {
	TV map[string]tvlist
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

type tvlist struct {
	Name string
	Host string
}

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

func showMessageAndRedirect(res http.ResponseWriter, message string, location string) {
	log.Printf("Redirecting: sending to %s\n", location)
	bodyHead := fmt.Sprintf("<html><head><meta http-equiv='refresh' content='5;url=%s'></head><body>", location)
	body := message
	bodyTail := "</body></html>"
	res.Write([]byte(bodyHead + body + bodyTail))
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

	formUsername := req.FormValue("username")
	formPassword := req.FormValue("password")

	log.Printf("Logging in with user: %s\n", formUsername)

	authenticated := LDAPAuthUser(formUsername, formPassword)

	if len(authenticated) > 0 {

		// this is not correct but i want to create a new struct that will have only
		// the allowed TVs used in the template
		for tvKey, tvValue := range Config.TV {
			for _, authValue := range authenticated {
				if tvKey == authValue {
					fmt.Printf("TVkey: %s, tvValue.Name: %s, tvValue.Host: %s\n", tvKey, tvValue.Name, tvValue.Host)
					validTV.TV.Name = tvValue.Name
					validTV.TV.Host = tvValue.Host
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

		log.Printf("login cookie set with token: %s\n", sessionToken.String())
		log.Println("/login redirect to /tv")
		http.Redirect(res, req, "/tv", 302)

	} else {
		log.Println("authenticated returned a zero size slice")
		http.Redirect(res, req, "/", 302)
	}

}

func tv(res http.ResponseWriter, req *http.Request) {
	c, err := req.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			log.Println("tv: cookie not set")
			res.WriteHeader(http.StatusUnauthorized)
			return
		}
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	sessionToken := c.Value

	log.Printf("tv: sessionToken: %s\n", sessionToken)

	t, err := template.New("webpage").Parse(tvTpl)
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

func sendURL(res http.ResponseWriter, req *http.Request) {
	c, err := req.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			log.Println("sendURL: cookie not set")
			res.WriteHeader(http.StatusUnauthorized)
			return
		}
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	sessionToken := c.Value

	log.Printf("sendURL: sessionToken: %s\n", sessionToken)

	tv := req.FormValue("tv")
	urlToBrowser := req.FormValue("url")

	log.Println(urlToBrowser)

	postData := url.Values{}
	postData.Set("u", urlToBrowser)

	//tvToSend := fmt.Sprintf("http://%s.hq.crosschx.com:%d/open", tv, Config.Remote.Port)
	tvToSend := fmt.Sprintf("http://%s:%d/open", tv, Config.Remote.Port)

	log.Printf("Sending the following: %v, %v\n", tvToSend, postData)

	resp, err := http.PostForm(tvToSend, postData)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	http.Redirect(res, req, "/tv", 302)

}

func refreshToken(res http.ResponseWriter, req *http.Request) {
	c, err := req.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			log.Println("refreshToken: cookie not set")
			res.WriteHeader(http.StatusUnauthorized)
			return
		}
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	sessionToken := c.Value

	log.Printf("Refresh: sessionToken: %s\n", sessionToken)

	newSessionToken, err := uuid.NewV4()
	http.SetCookie(res, &http.Cookie{
		Name:    "session_token",
		Value:   newSessionToken.String(),
		Expires: time.Now().Add(120 * time.Second),
	})
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
	http.HandleFunc("/tv", tv)
	http.HandleFunc("/sendurl", sendURL)
	http.HandleFunc("/refresh", refreshToken)

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
