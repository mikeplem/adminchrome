#!/bin/bash
env GOOS=linux GOARCH=arm CGO_ENABLED=0 go build -o adminchrome_arm main.go ldap.go
