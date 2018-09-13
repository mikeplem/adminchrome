package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"time"

	ldap "gopkg.in/ldap.v2"
)

// LDAPAuthUser - authenticate the user to LDAP
// to verify they have a valid account before
// any database account can be created
func LDAPAuthUser(username string, password string, tv string) bool {

	// Connect to the LDAP server
	l, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", Config.LDAP.Host, Config.LDAP.Port))
	if err != nil {
		fmt.Print(time.Now())
		fmt.Println(err)
		return false
	}
	defer l.Close()

	// Reconnect with TLS
	err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		fmt.Print(time.Now())
		fmt.Println(err)
		return false
	}

	// Connect with the user to verify the account is valid
	err = l.Bind(Config.LDAP.BindDN, Config.LDAP.BindPassword)
	if err != nil {
		fmt.Print(time.Now())
		fmt.Println(err)
		fmt.Println(Config.LDAP.BindDN)
		return false
	}

	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		Config.LDAP.Base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(uid=%s)(|(memberOf=cn=hqtv%s,%s)(memberOf=cn=hqtvall,%s)))", username, tv, Config.LDAP.GroupBase, Config.LDAP.GroupBase),
		[]string{"dn"},
		nil,
	)

	// Search for the user and being a member of the TV
	sr, err := l.Search(searchRequest)
	if err != nil {
		log.Fatal(err)
		return false
	}

	if len(sr.Entries) != 1 {
		log.Fatal("User does not exist or too many entries returned")
		return false
	}

	userdn := sr.Entries[0].DN

	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		log.Fatal(err)
		return false
	}

	return true
}
