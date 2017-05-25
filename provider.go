package main

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/peterbale/go-phpipam"
)

// Client struct for all Terraform provider methods
type Client struct {
	PhpIPAMClient *phpipam.Client
}

// Provider method to define all user inputs
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"server_url": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHPIPAM_SERVER_URL", nil),
				Description: "phpIPAM REST API Server URL",
			},
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHPIPAM_USERNAME", nil),
				Description: "Username",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHPIPAM_PASSWORD", nil),
				Description: "Password",
			},
			"ssl_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHPIPAM_SSL_SKIP_VERIFY", nil),
				Description: "Skip SSL verify check",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"phpipam_address": resourcePhpIPAMAddress(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	config := &phpipam.Config{
		Hostname:      d.Get("server_url").(string),
		Username:      d.Get("username").(string),
		Password:      d.Get("password").(string),
		SSLSkipVerify: d.Get("ssl_skip_verify").(bool),
		Application:   "terraform",
	}
	phpIPAMClient, err := config.NewClient()
	if err != nil {
		return nil, fmt.Errorf("Error setting up phpIPAM client: %s", err)
	}
	log.Printf("[INFO] phpIPAM Client configured for server %s", config.Hostname)
	client := &Client{
		PhpIPAMClient: phpIPAMClient,
	}
	return client, nil
}
