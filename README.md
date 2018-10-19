# Administrate the Remote Control of Chromium

This program is used as a central front end to open a URL of a Chromium browser being controlled by the remotechrome program.  Authentication is handled via LDAP.

## Depends

This program depends upon

* github.com/BurntSushi/toml

* github.com/go-ldap/ldap

* github.com/satori/go.uuid

Auth help from https://www.sohamkamani.com/blog/2018/03/25/golang-session-authentication/

## Requirements

You must fill in the necessary items in the config.toml file.

### Building

Use the appropriate build_ARCH.sh script

### Server

* The listen port for the adminchrome web server
* If you want to enable SSL or not
* The path to the necessary SSL certs if you want SSL

### LDAP

* Do you desire authentication or not
* The LDAP server address
* The LDAP server port
* The base DN to search for users
* The group base DN to search for TVs allowed to be accessed by user
* The bind DN username
* The bind DN password

## Why

The idea is that this program is the basis for remotely handling controlling a browser that would be used as a digital sign.

## Launching

I was looking for a simple way to remotely open a URL in Chromium. 

```shell
$ ./adminchrome_mac -help
Usage of ./adminchrome_mac:
  -conf string
        Config file for this listener and ldap configs
```

```shell
./adminchrome_mac -conf config.toml
```

## Config TOML Format

### Listen Section

This section is used to configure the port you want the web interface for this tool to listen.  It is also where you configure SSL support.

```shell
[listen]
ssl = false
cert = "wildcard_certificate"
key = "wildcard_key"
port = 8081
```

### Remote Section

This is the port the remote Chromium client will listen

```shell
[remote]
port = 8080
```

### LDAP Section

Here you configure the following:

* `useldap` is a boolean which will either enable or disable authentication
* LDAP Server and port: `host` and `port`
* `base` is the LDAP DN where user accounts will be searched for.
* `groupbase` is the LDAP group DN where the search query will look for any group that a user is a member of
* `binddn` and `bindpassword` is the bind user that will authenticate to LDAP to perform the user search

If authentitcation is disabled then all TVs listed in the configuration toml will be in the selection dropdown.

The code is written to attempt to use StartTLS for the LDAP connection.

The `memberOf` attribute is used to determine group membership.

```shell
[ldap]
useldap = true
host = "ldap.example.com"
port = 389
base = "cn=users,cn=accounts,dc=example,dc=com"
groupbase = "cn=groups,cn=accounts,dc=example,dc=com"
binddn = "cn=users,cn=accounts,dc=example,dc=com"
bindpassword = "some password"
```

### TV Section

The part that matters most is the subsection.  In this case it would be `tv.hqtv1`, `tv.hqtv2`, `tv.hqtv3`.

The `hqtvX` text is the LDAP group name that a user must be a member of.

Effectively, this is the ldapsearch query happening.

```shell
ldapsearch -x -D "BIND_DN_USER" -W -b "cn=users,cn=accounts,dc=example,dc=com" "uid=USER" dn memberOf
```

The group membership would look like the following

```shell
memberOf: cn=hqtv1,cn=groups,cn=accounts,dc=example,dc=com
memberOf: cn=hqtvall,cn=groups,cn=accounts,dc=example,dc=com
```

The LDAP `groupbase` is used to strip that part of the DN off the LDAP query results so that only the actual group name is returned and used to build the page used for TV selection for sending a URL.

The `name` field is used as the text of the drop down and the `host` field is used for the value of drop down.

```shell
[tv]

	[tv.hqtv1]
	name = "TV 1"
	host = "localhost"

	[tv.hqtv2]
	name = "TV 2"
	host = "tv2.example.com"

	[tv.hqtv3]
	name = "TV 3"
	host = "tv3.example.com"
```
