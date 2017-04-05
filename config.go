package main

import (
	"fmt"
	"log"

	"github.com/peterbale/go-phpipam"
)

type Config struct {
	ServerUrl string
	Username  string
	Password  string
}

func (c *Config) Client() (*phpipam.Client, error) {
	client, err := phpipam.New(c.ServerUrl, "terraform", c.Username, c.Password)
	if err != nil {
		return nil, fmt.Errorf("Error setting up phpIPAM client: %s", err)
	}
	log.Printf("[INFO] phpIPAM Client configured for server %s", c.ServerUrl)
	return client, nil
}
