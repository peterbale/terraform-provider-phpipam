package main

import (
	"log"

	"github.com/hashicorp/terraform/helper/schema"
)

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
				Description: "phpIPAM Username",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHPIPAM_PASSWORD", nil),
				Description: "phpIPAM Password",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"phpipam_address": resourcePhpIPAMAddress(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	config := Config{
		ServerUrl: d.Get("server_url").(string),
		Username:  d.Get("username").(string),
		Password:  d.Get("password").(string),
	}

	log.Println("[INFO] Initializing phpIPAM client")
	return config.Client()
}
