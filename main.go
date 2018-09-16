package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/BurntSushi/toml"
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

type tvData struct {
	TVStuff []tvlist
}

var data tvData

func showMessageAndRedirect(res http.ResponseWriter, message string) {
	bodyHead := "<html><head><meta http-equiv='refresh' content='5;url=/login.html'></head><body>"
	body := message
	bodyTail := "</body></html>"
	res.Write([]byte(bodyHead + body + bodyTail))
}

func homePage(res http.ResponseWriter, req *http.Request) {
	fmt.Println("login.html requested")
	http.ServeFile(res, req, "login.html")
}

func login(res http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		fmt.Println("login.html requested")
		http.ServeFile(res, req, "login.html")
		return
	}

	formUsername := req.FormValue("username")
	formPassword := req.FormValue("password")
	tv := req.FormValue("tv")
	urlToBrowser := req.FormValue("url")

	fmt.Println(urlToBrowser)

	postData := url.Values{}
	postData.Set("u", urlToBrowser)

	authenticated := LDAPAuthUser(formUsername, formPassword, tv)

	if authenticated {

		tvToSend := fmt.Sprintf("http://%s.hq.crosschx.com:%d/open", tv, Config.Remote.Port)

		fmt.Printf("Sending the following: %v, %v\n", tvToSend, postData)

		resp, err := http.PostForm(tvToSend, postData)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		returnMessage := fmt.Sprintf("%s was sent to TV %s", postData, tvToSend)
		showMessageAndRedirect(res, returnMessage)

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

	data = tvData{
		TVStuff: []tvlist{
			{Name: "TV 1", Host: "tv1"},
			{Name: "TV 2", Host: "tv2"},
			{Name: "TV 3", Host: "tv3"},
			{Name: "TV 4", Host: "tv4"},
			{Name: "TV 5", Host: "tv5"},
			{Name: "TV 6", Host: "tv6"},
			{Name: "TV 7", Host: "tv7"},
		},
	}

	ParseTemplate("sendurl.template", "sendurl.html")

}

func main() {

	// Setup the proper format for the listening port on any interface
	listenPort := fmt.Sprintf(":%d", Config.Listen.Port)

	http.HandleFunc("/", homePage)
	http.HandleFunc("/login", login)

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
