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

### Server

* The listen port for the adminchrome web server
* If you want to enable SSL or not
* The path to the necessary SSL certs if you want SSL

### LDAP

* The LDAP server address
* The LDAP server port
* The base DN to search for users
* The bind DN username
* The bind DN password

## Why

The idea is that this program is the basis for remotely handling controlling a browser that would be used as a digital sign.

## Launching

I was looking for a simple way to remotely open a URL in Chromium.  The Go code takes two arguments.

```shell
$ ./adminchrome -help
Usage of ./remotechrome:
  -conf
        Config file for this listener and ldap configs
```

## Usage

