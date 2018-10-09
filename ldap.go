package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"strings"

	ldap "gopkg.in/ldap.v2"
)

// LDAPAuthUser - authenticate the user to LDAP
// to verify they have a valid account before
// any database account can be created
//func LDAPAuthUser(username string, password string, tv string) bool {
func LDAPAuthUser(username string, password string) []string {

	// Connect to the LDAP server
	l, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", Config.LDAP.Host, Config.LDAP.Port))
	if err != nil {
		log.Print(err)
		return allowedTvs
	}
	defer l.Close()

	// Reconnect with TLS
	err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Print(err)
		return allowedTvs
	}

	// Connect with the user to verify the account is valid
	err = l.Bind(Config.LDAP.BindDN, Config.LDAP.BindPassword)
	if err != nil {
		log.Print(err)
		log.Print(Config.LDAP.BindDN)
		return allowedTvs
	}

	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		Config.LDAP.Base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 10, 0, false,
		fmt.Sprintf("(&(uid=%s))", username),
		[]string{"dn", "memberOf"},
		nil,
	)

	// Search for the user and being a member of the TV
	sr, err := l.Search(searchRequest)
	if err != nil {
		log.Print("Search failure: ", err)
		return allowedTvs
	}

	if len(sr.Entries) != 1 {
		log.Print("User does not exist or too many entries returned")
		return allowedTvs
	}

	userdn := sr.Entries[0].DN

	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		log.Print(err)
		return allowedTvs
	}

	for _, entry := range sr.Entries {
		for _, foo := range entry.GetAttributeValues("memberOf") {
			memberAttr := fmt.Sprint(foo)
			if strings.Contains(memberAttr, "cn=hqtv") {
				removePrefix := strings.TrimPrefix(memberAttr, "cn=")
				removeSuffix := strings.TrimSuffix(removePrefix, ",cn=groups,cn=accounts,dc=mgmt,dc=crosschx,dc=com")
				allowedTvs = append(allowedTvs, removeSuffix)
			}
		}
	}

	return allowedTvs
}
