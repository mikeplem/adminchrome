package main

import (
	"html/template"
	"log"
	"os"
)

// ParseTemplate and write to file
func ParseTemplate(templateName string, fileName string) {
	t, err := template.ParseFiles(templateName)
	if err != nil {
		log.Print(err)
		return
	}

	f, err := os.Create(fileName)
	if err != nil {
		log.Println("create file: ", err)
		return
	}

	err = t.Execute(f, data)
	if err != nil {
		log.Print("execute: ", err)
		return
	}
	f.Close()
}
