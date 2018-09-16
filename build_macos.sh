#!/bin/bash
env GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o adminchrome_mac main.go ldap.go template.go
