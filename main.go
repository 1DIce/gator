package main

import (
	"fmt"
	"log"

	"github.com/1DIce/gator/internal/config"
)

func main() {
	configFile, err := config.Read()
	if err != nil {
		log.Fatal("No config file was found")
	}

	configFile.CurrentUserName = "lars"

	if err := config.Write(configFile); err != nil {
		log.Fatal("Failed to write config file")
	}

	newConfig, err := config.Read()
	if err != nil {
		log.Fatal("Failed to read config again!")
	}

	fmt.Print(newConfig)
}
