#!/bin/bash
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o adminchrome_linux main.go ldap.go

