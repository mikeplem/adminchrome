package main

import (
	"flag"
	"fmt"
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

type tomlConfig struct {
	Listen listenconfig `toml:"listen"`
	LDAP   ldapconfig   `toml:"ldap"`
	Remote remoteconfig `toml:"remote"`
	TV     map[string]tvlist
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

func showMessageAndRedirect(res http.ResponseWriter, message string) {
	bodyHead := "<html><head><meta http-equiv='refresh' content='5;url=/login.html'></head><body>"
	body := message
	bodyTail := "</body></html>"
	res.Write([]byte(bodyHead + body + bodyTail))
}

func homePage(res http.ResponseWriter, req *http.Request) {
	fmt.Println("loging.html requested")
	http.ServeFile(res, req, "login.html")
}

func refreshToken(res http.ResponseWriter, req *http.Request) {
	c, err := req.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			res.WriteHeader(http.StatusUnauthorized)
			return
		}
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	sessionToken := c.Value

	fmt.Printf("Refresh: sessionToken: %s\n", sessionToken)

	// Now, create a new session token for the current user
	newSessionToken, err := uuid.NewV4()

	// Set the new token as the users `session_token` cookie
	http.SetCookie(res, &http.Cookie{
		Name:    "session_token",
		Value:   newSessionToken.String(),
		Expires: time.Now().Add(120 * time.Second),
	})
}

func sendURL(res http.ResponseWriter, req *http.Request) {
	// We can obtain the session token from the requests cookies, which come with every request
	c, err := req.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			// If the cookie is not set, return an unauthorized status
			res.WriteHeader(http.StatusUnauthorized)
			return
		}
		// For any other type of error, return a bad request status
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	sessionToken := c.Value

	fmt.Printf("sendURL: sessionToken: %s\n", sessionToken)

	tv := req.FormValue("tv")
	urlToBrowser := req.FormValue("url")

	fmt.Println(urlToBrowser)

	postData := url.Values{}
	postData.Set("u", urlToBrowser)

	//tvToSend := fmt.Sprintf("http://%s.hq.crosschx.com:%d/open", tv, Config.Remote.Port)
	tvToSend := fmt.Sprintf("http://%s:%d/open", tv, Config.Remote.Port)

	fmt.Printf("Sending the following: %v, %v\n", tvToSend, postData)

	resp, err := http.PostForm(tvToSend, postData)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	returnMessage := fmt.Sprintf("%s was sent to TV %s", postData, tvToSend)
	showMessageAndRedirect(res, returnMessage)

}

func login(res http.ResponseWriter, req *http.Request) {

	formUsername := req.FormValue("username")
	formPassword := req.FormValue("password")
	tv := req.FormValue("tv")

	authenticated := LDAPAuthUser(formUsername, formPassword, tv)

	if authenticated {

		// create session token for user
		sessionToken, err := uuid.NewV4()
		if err != nil {
			fmt.Printf("sessionToken failed to create: %s", err)
			return
		}

		// Finally, we set the client cookie for "session_token" as the session token we just generated
		// we also set an expiry time of 120 seconds
		http.SetCookie(res, &http.Cookie{
			Name:    "session_token",
			Value:   sessionToken.String(),
			Expires: time.Now().Add(120 * time.Second),
		})

		fmt.Println("sendurl.html requested")
		http.ServeFile(res, req, "sendurl.html")

	} else {
		showMessageAndRedirect(res, "Failed to authenticate to LDAP for user: "+formUsername)
	}

}

func init() {

	ConfigFile = flag.String("conf", "", "Config file for this listener and ldap configs")

	flag.Parse()

	if _, err := toml.DecodeFile(*ConfigFile, &Config); err != nil {
		log.Fatal(err)
	}

	ParseTemplate("sendurl.template", "sendurl.html")

}

func main() {

	// Setup the proper format for the listening port on any interface
	listenPort := fmt.Sprintf(":%d", Config.Listen.Port)

	http.HandleFunc("/", homePage)
	http.HandleFunc("/login", login)
	http.HandleFunc("/sendurl", sendURL)
	http.HandleFunc("/refresh", refreshToken)

	if Config.Listen.SSL == true {
		fmt.Println("Listening on port " + listenPort + " with SSL")
		err := http.ListenAndServeTLS(listenPort, Config.Listen.Cert, Config.Listen.Key, nil)
		if err != nil {
			fmt.Print(time.Now())
			log.Fatal("ListenAndServe: ", err)
		}
	} else {
		fmt.Println("Listening on port " + listenPort + " without SSL")
		err := http.ListenAndServe(listenPort, nil)
		if err != nil {
			fmt.Print(time.Now())
			log.Fatal("ListenAndServe: ", err)
		}
	}

}
