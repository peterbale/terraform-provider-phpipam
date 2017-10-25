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

// AddressInformation struct to define ipaddress data
type AddressInformation struct {
	Hostname  string
	IP        string
	Section   string
	Subnet    string
	Broadcast string
	Gateway   string
	BitMask   string
	Index     string
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
			"index": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func (c *Client) findSectionID(section string) (string, error) {
	var sectionID string
	sections, err := c.PhpIPAMClient.GetSections()
	if err != nil {
		return sectionID, err
	}
	for _, element := range sections.Data {
		if element.Name == section {
			sectionID = element.ID
		}
	}
	if len(sectionID) == 0 {
		return sectionID, errors.New("Section Not Found")
	}
	return sectionID, nil
}

func (c *Client) findSubnetID(sectionID string, subnet string) (string, error) {
	var subnetID string
	subnets, err := c.PhpIPAMClient.GetSectionsSubnets(sectionID)
	if err != nil {
		return subnetID, err
	}
	for _, element := range subnets.Data {
		if element.Description == subnet {
			subnetID = element.ID
		}
	}
	if len(subnetID) == 0 {
		return subnetID, errors.New("Subnet Not Found")
	}
	return subnetID, nil
}

func (c *Client) findExistingAddress(hostname string, index string) (string, error) {
	var address string
	var totalFoundAddresses int
	addresses, err := c.PhpIPAMClient.GetAddressSearch(hostname)
	if err != nil {
		return address, err
	}
	totalFoundAddresses = len(addresses.Data)
	if totalFoundAddresses == 0 {
		return address, nil
	}
	if len(index) > 0 {
		totalFoundAddresses = 0
		for _, i := range addresses.Data {
			if i.Description == index {
				totalFoundAddresses++
				if totalFoundAddresses > 1 {
					return address, errors.New("Multiple Indexed Addresses Found")
				}
				address = i.IP
			}
		}
	} else if totalFoundAddresses > 1 {
		return address, errors.New("Multiple Addresses Found")
	} else {
		address = addresses.Data[0].IP
	}
	return address, nil
}

func (c *Client) getAddressID(address string) (string, error) {
	var addressID string
	addressSearchIP, err := c.PhpIPAMClient.GetAddressSearchIP(address)
	if err != nil {
		return addressID, err
	}
	if len(addressSearchIP.Data) != 1 {
		return addressID, errors.New("Address Over Allocated")
	}
	return addressSearchIP.Data[0].ID, nil
}

func (c *Client) getAddressInformation(addressID string) (*AddressInformation, error) {
	addressInformation := new(AddressInformation)
	var subnetID, sectionID string
	addressData, err := c.PhpIPAMClient.GetAddress(addressID)
	if err != nil {
		return nil, err
	}
	if addressData.Code == 200 {
		addressInformation.Hostname = addressData.Data.Hostname
		addressInformation.IP = addressData.Data.IP
		subnetID = addressData.Data.SubnetID
	} else {
		return nil, nil
	}
	addressSearchData, err := c.PhpIPAMClient.GetAddressSearch(addressInformation.Hostname)
	if err != nil {
		return nil, err
	}
	if addressSearchData.Code == 200 {
		for _, address := range addressSearchData.Data {
			if address.ID == addressID {
				_, err = strconv.Atoi(address.Description)
				if err == nil {
					addressInformation.Index = address.Description
				}
			}
		}
	}
	subnetData, err := c.PhpIPAMClient.GetSubnet(subnetID)
	if err != nil {
		return nil, err
	}
	if subnetData.Code == 200 {
		addressInformation.Subnet = subnetData.Data.Description
		addressInformation.Broadcast = subnetData.Data.Calculation.Broadcast
		addressInformation.Gateway = subnetData.Data.Gateway.IPAddress
		addressInformation.BitMask = subnetData.Data.Calculation.BitMask
		sectionID = subnetData.Data.SectionID
	} else {
		return nil, errors.New("Address Subnet Not Found")
	}
	sectionData, err := c.PhpIPAMClient.GetSection(sectionID)
	if err != nil {
		return nil, err
	}
	if sectionData.Code == 200 {
		addressInformation.Section = sectionData.Data.Name
	} else {
		return nil, errors.New("Subnet Section Not Found")
	}
	return addressInformation, nil
}

func checkAddressSubnet(existingSubnetID string, subnetID string) int {
	var subnetMatchBool int
	if existingSubnetID == subnetID {
		subnetMatchBool = 1
	} else {
		subnetMatchBool = 0
	}
	return subnetMatchBool
}

func (c *Client) allocateNewAddress(subnetID string, hostname string, index string) (phpipam.AddressFirstFree, error) {
	newAddress, err := c.PhpIPAMClient.CreateAddressFirstFree(subnetID, hostname, "terraform", index)
	if err != nil {
		return newAddress, err
	}
	return newAddress, nil
}

func (c *Client) deleteExistingAddress(addressID string) (phpipam.AddressDelete, error) {
	deleteAddress, err := c.PhpIPAMClient.DeleteAddress(addressID)
	if err != nil {
		return deleteAddress, err
	}
	return deleteAddress, nil
}

func (c *Client) create(section string, subnet string, hostname string, update bool, index string) (string, error) {
	addonLock.Lock()
	defer addonLock.Unlock()

	var addressID string
	sectionID, err := c.findSectionID(section)
	if err != nil {
		return addressID, fmt.Errorf("Error Getting Section ID: %s", err)
	}
	subnetID, err := c.findSubnetID(sectionID, subnet)
	if err != nil {
		return addressID, fmt.Errorf("Error Getting Subnet ID: %s", err)
	}
	address, err := c.findExistingAddress(hostname, index)
	if err != nil {
		return addressID, fmt.Errorf("Error Finding Existing Addresses: %s", err)
	}
	if len(address) == 0 || update {
		log.Printf("[DEBUG] New Address Section ID: %#v, Subnet ID: %#v", sectionID, subnetID)
		newAddress, err := c.allocateNewAddress(subnetID, hostname, index)
		if err != nil {
			return addressID, fmt.Errorf("Error Allocating New Address: %s", err)
		}
		log.Printf("[DEBUG] New Address IP: %#v", newAddress)
		addressID, err = c.getAddressID(newAddress.IP)
		if err != nil {
			return addressID, fmt.Errorf("Error Getting Created Address ID: %s", newAddress.IP)
		}
		log.Printf("[INFO] New Address Allocated: %s", newAddress.IP)
	} else {
		log.Printf("[DEBUG] Existing Address Section ID: %#v, Subnet ID: %#v", sectionID, subnetID)
		log.Printf("[DEBUG] Existing Address IP: %#v", address)
		addressID, err = c.getAddressID(address)
		if err != nil {
			return addressID, fmt.Errorf("Error Getting Created Address ID: %s", address)
		}
		log.Printf("[INFO] New Address Allocated: %s", address)
	}
	return addressID, nil
}

func (c *Client) delete(addressID string, update bool) error {
	_, err := c.deleteExistingAddress(addressID)
	if err != nil {
		return fmt.Errorf("Delete Address Failed: %s", err)
	}
	log.Printf("[INFO] Address Removed: %s", addressID)
	return nil
}

func resourcePhpIPAMAddressCreate(d *schema.ResourceData, m interface{}) error {
	section := d.Get("section").(string)
	subnet := d.Get("subnet").(string)
	hostname := d.Get("hostname").(string)
	index := d.Get("index").(string)
	client := m.(*Client)
	addressID, err := client.create(section, subnet, hostname, false, index)
	if err != nil {
		return err
	}
	d.SetId(addressID)
	return resourcePhpIPAMAddressRead(d, m)
}

func resourcePhpIPAMAddressRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*Client)
	log.Printf("[INFO] Address ID Created: %s", d.Id())
	addressInformation, err := client.getAddressInformation(d.Id())
	if err != nil {
		return fmt.Errorf("Cannot Get Address Infomation: %s", err)
	}
	d.Set("hostname", addressInformation.Hostname)
	d.Set("section", addressInformation.Section)
	d.Set("subnet", addressInformation.Subnet)
	d.Set("ip_address", addressInformation.IP)
	d.Set("broadcast", addressInformation.Broadcast)
	d.Set("gateway", addressInformation.Gateway)
	d.Set("bitmask", addressInformation.BitMask)
	d.Set("index", addressInformation.Index)
	return nil
}

func resourcePhpIPAMAddressrUpdate(d *schema.ResourceData, m interface{}) error {
	section := d.Get("section").(string)
	subnet := d.Get("subnet").(string)
	hostname := d.Get("hostname").(string)
	index := d.Get("index").(string)
	client := m.(*Client)
	addressID := d.Id()
	var err error
	if d.HasChange("hostname") {
		_, err = client.PhpIPAMClient.PatchUpdateAddress(hostname, addressID)
		if err != nil {
			return fmt.Errorf("Address Update Failed: %s", err)
		}
		log.Printf("[INFO] Address Updated: %s", hostname)
	} else {
		newAddressID, err := client.create(section, subnet, hostname, true, index)
		if err != nil {
			return err
		}
		err = client.delete(addressID, true)
		if err != nil {
			return err
		}
		d.SetId(newAddressID)
	}
	return resourcePhpIPAMAddressRead(d, m)
}

func resourcePhpIPAMAddressDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*Client)
	err := client.delete(d.Id(), false)
	return err
}
