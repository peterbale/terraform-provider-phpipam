package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/peterbale/go-phpipam"
)

var addonLock sync.Mutex

type AddressInformation struct {
	Hostname  string
	Ip        string
	Section   string
	Subnet    string
	Broadcast string
	Gateway   string
	BitMask   string
}

func resourcePhpIPAMAddress() *schema.Resource {
	return &schema.Resource{
		Create: resourcePhpIPAMAddressCreate,
		Read:   resourcePhpIPAMAddressRead,
		Update: resourcePhpIPAMAddressrUpdate,
		Delete: resourcePhpIPAMAddressDelete,

		Schema: map[string]*schema.Schema{
			"hostname": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"section": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"subnet": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"ip_address": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"broadcast": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"gateway": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"bitmask": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func findSectionId(client *phpipam.Client, section string) (string, error) {
	var sectionId string
	sections, err := client.GetSections()
	if err != nil {
		return sectionId, err
	}
	for _, element := range sections.Data {
		if element.Name == section {
			sectionId = element.Id
		}
	}
	if len(sectionId) == 0 {
		return sectionId, errors.New("Section Not Found")
	}
	return sectionId, nil
}

func findSubnetId(client *phpipam.Client, sectionId string, subnet string) (string, error) {
	var subnetId string
	subnets, err := client.GetSectionsSubnets(sectionId)
	if err != nil {
		return subnetId, err
	}
	for _, element := range subnets.Data {
		if element.Description == subnet {
			subnetId = element.Id
		}
	}
	if len(subnetId) == 0 {
		return subnetId, errors.New("Subnet Not Found")
	}
	return subnetId, nil
}

func findExistingAddress(client *phpipam.Client, hostname string) (phpipam.AddressSearch, int, error) {
	var totalFoundAddresses int
	addresses, err := client.GetAddressSearch(hostname)
	if err != nil {
		return addresses, totalFoundAddresses, err
	}
	totalFoundAddresses = len(addresses.Data)
	return addresses, totalFoundAddresses, nil
}

func getAddressId(client *phpipam.Client, address string) (string, error) {
	var addressId string
	addressSearchIp, err := client.GetAddressSearchIp(address)
	if err != nil {
		return addressId, err
	}
	if len(addressSearchIp.Data) != 1 {
		return addressId, errors.New("Address Over Allocated")
	}
	return addressSearchIp.Data[0].Id, nil
}

func getAddressInformation(client *phpipam.Client, addressId string) (*AddressInformation, error) {
	var hostname, subnetId, subnet, sectionId, section, address, broadcast, gateway, bitmask string
	addressData, err := client.GetAddress(addressId)
	if err != nil {
		return nil, err
	}
	if addressData.Code == 200 {
		hostname = addressData.Data.Hostname
		subnetId = addressData.Data.SubnetId
		address = addressData.Data.Ip
	} else {
		return nil, nil
	}
	subnetData, err := client.GetSubnet(subnetId)
	if err != nil {
		return nil, err
	}
	if subnetData.Code == 200 {
		subnet = subnetData.Data.Description
		sectionId = subnetData.Data.SectionId
		broadcast = subnetData.Data.Calculation.Broadcast
		gateway = subnetData.Data.Gateway.IPAddress
		bitmask = subnetData.Data.Calculation.BitMask
	} else {
		return nil, errors.New("Address Subnet Not Found")
	}
	sectionData, err := client.GetSection(sectionId)
	if err != nil {
		return nil, err
	}
	if sectionData.Code == 200 {
		section = sectionData.Data.Name
	} else {
		return nil, errors.New("Subnet Section Not Found")
	}
	return &AddressInformation{
		Hostname:  hostname,
		Ip:        address,
		Section:   section,
		Subnet:    subnet,
		Broadcast: broadcast,
		Gateway:   gateway,
		BitMask:   bitmask,
	}, nil
}

func checkAddressLive(client *phpipam.Client, addressId string) (int, error) {
	var pingStatusBool int
	pingStatus, err := client.GetAddressPing(addressId)
	if err != nil {
		return pingStatusBool, err
	}
	if pingStatus.Code == 200 {
		pingStatusBool = 1
	} else {
		pingStatusBool = 0
	}
	return pingStatusBool, nil
}

func checkAddressSubnet(existingSubnetId string, subnetId string) int {
	var subnetMatchBool int
	if existingSubnetId == subnetId {
		subnetMatchBool = 1
	} else {
		subnetMatchBool = 0
	}
	return subnetMatchBool
}

func allocateNewAddress(client *phpipam.Client, subnetId string, hostname string) (phpipam.AddressFirstFree, error) {
	newAddress, err := client.CreateAddressFirstFree(subnetId, hostname, "terraform")
	if err != nil {
		return newAddress, err
	}
	return newAddress, nil
}

func deleteExistingAddress(client *phpipam.Client, addressId string) (phpipam.AddressDelete, error) {
	deleteAddress, err := client.DeleteAddress(addressId)
	if err != nil {
		return deleteAddress, err
	}
	return deleteAddress, nil
}

func create(client *phpipam.Client, section string, subnet string, hostname string, update bool) (string, error) {
	addonLock.Lock()
	defer addonLock.Unlock()

	var addressId string
	var err error
	sectionId, err := findSectionId(client, section)
	if err != nil {
		return addressId, fmt.Errorf("Error Getting Section ID: %s", err)
	}
	subnetId, err := findSubnetId(client, sectionId, subnet)
	if err != nil {
		return addressId, fmt.Errorf("Error Getting Subnet ID: %s", err)
	}
	_, totalFoundAddresses, err := findExistingAddress(client, hostname)
	if err != nil {
		return addressId, fmt.Errorf("Error Finding Existing Addresses: %s", err)
	}
	if totalFoundAddresses == 0 || (totalFoundAddresses == 1 && update) {
		log.Printf("[DEBUG] New Address Section ID: %#v, Subnet ID: %#v", sectionId, subnetId)
		newAddress, err := allocateNewAddress(client, subnetId, hostname)
		if err != nil {
			return addressId, fmt.Errorf("Error Allocating New Address: %s", err)
		}
		log.Printf("[DEBUG] New Address IP: %#v", newAddress)
		addressId, err = getAddressId(client, newAddress.Ip)
		if err != nil {
			return addressId, fmt.Errorf("Error Getting Created Address ID: %s", newAddress.Ip)
		}
		log.Printf("[INFO] New Address Allocated: %s", newAddress.Ip)
	} else {
		return addressId, fmt.Errorf("Error Address Already Allocated, Total Found Addresses: %s", strconv.Itoa(totalFoundAddresses))
	}
	return addressId, nil
}

func delete(client *phpipam.Client, addressId string, update bool) error {
	if !update {
		addressState, err := checkAddressLive(client, addressId)
		if err != nil {
			return fmt.Errorf("Address Liveliness Check Failed: %s", err)
		} else if addressState != 0 {
			return fmt.Errorf("Address Host is Still Live")
		}
	}
	_, err := deleteExistingAddress(client, addressId)
	if err != nil {
		return fmt.Errorf("Delete Address Failed: %s", err)
	}
	log.Printf("[INFO] Address Removed: %s", addressId)
	return nil
}

func resourcePhpIPAMAddressCreate(d *schema.ResourceData, m interface{}) error {
	section := d.Get("section").(string)
	subnet := d.Get("subnet").(string)
	hostname := d.Get("hostname").(string)
	client := m.(*phpipam.Client)
	addressId, err := create(client, section, subnet, hostname, false)
	if err != nil {
		return err
	}
	d.SetId(addressId)
	return resourcePhpIPAMAddressRead(d, m)
}

func resourcePhpIPAMAddressRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*phpipam.Client)
	log.Printf("[INFO] Address ID Created: %s", d.Id())
	addressInformation, err := getAddressInformation(client, d.Id())
	if err != nil {
		return fmt.Errorf("Cannot Get Address Infomation: %s", err)
	}
	d.Set("hostname", addressInformation.Hostname)
	d.Set("section", addressInformation.Section)
	d.Set("subnet", addressInformation.Subnet)
	d.Set("ip_address", addressInformation.Ip)
	d.Set("broadcast", addressInformation.Broadcast)
	d.Set("gateway", addressInformation.Gateway)
	d.Set("bitmask", addressInformation.BitMask)
	return nil
}

func resourcePhpIPAMAddressrUpdate(d *schema.ResourceData, m interface{}) error {
	section := d.Get("section").(string)
	subnet := d.Get("subnet").(string)
	hostname := d.Get("hostname").(string)
	client := m.(*phpipam.Client)
	addressId := d.Id()
	var err error
	if d.HasChange("hostname") {
		_, err = client.PatchUpdateAddress(hostname, addressId)
		if err != nil {
			return fmt.Errorf("Address Update Failed: %s", err)
		}
		log.Printf("[INFO] Address Updated: %s", hostname)
	} else {
		newAddressId, err := create(client, section, subnet, hostname, true)
		if err != nil {
			return err
		}
		err = delete(client, addressId, true)
		if err != nil {
			return err
		}
		d.SetId(newAddressId)
	}
	return resourcePhpIPAMAddressRead(d, m)
}

func resourcePhpIPAMAddressDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*phpipam.Client)
	err := delete(client, d.Id(), false)
	return err
}
