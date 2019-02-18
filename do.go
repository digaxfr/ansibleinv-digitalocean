package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

const ansibleGroupPrefix string = "do_"
const doApi string = "https://api.digitalocean.com/v2"

var ai = ansibleInventory{}
var environVars = map[string]string{
	"DO_TOKEN": "The Digital Ocean token API access key",
}

type doClient struct {
	api   string
	token string
}

// The response from querying /droplets
type dropletsResponse struct {
	Droplets []droplet `json:"droplets"`
	Links    struct {
		First string `json:"first,omitempty"`
		Prev  string `json:"prev,omitempty"`
		Next  string `json:"next,omitempty"`
		Last  string `json:"last,omitempty"`
	} `json:"links"`
	Meta struct {
		Total int `json:"total,omitempty"`
	} `json:"meta"`
}

// The structure of a droplet with only the relelvant fields for inventory generation
type droplet struct {
	Id       int      `json:"id"`
	Name     string   `json:"name"`
	Features []string `json:"features"`
	Region   struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"region"`
	Image struct {
		id   int    `json:"id"`
		name string `json:"name"`
	} `json:"image"`
	Networks struct {
		V4 []struct {
			IpAddress string `json:"ip_address"`
			Netmask   string `json:"netmask"`
			Gateway   string `json:"gateway"`
			Type      string `json:"type"`
		} `json:"v4"`
		V6 []struct {
			IpAddress string `json:"ip_address"`
			Netmask   int    `json:"netmask"`
			Gateway   string `json:"gateway"`
			Type      string `json:"type"`
		}
	} `json:"networks"`
	Tags []string `json:"tags"`
}

// Top level Ansible Inventory
// Hostvars has to be interface because we cannot ensure the structure of the data (could be strings, could be more
// nested structures, etc.)
type ansibleInventory struct {
	Groups map[string]*ansibleGroup `json:"groups"`
	Meta   struct {
		Hostvars map[string]interface{} `json:"hostvars"`
	} `json:"_meta"`
}

// Individual hostgroup/groupvar, easier to manipulate than it being nested inside ansibleInventory
// Vars has to be interface because we cannot ensure the structure of the data (could be strings, could be more nested
// structures, etc.)
type ansibleGroup struct {
	Hosts    []string               `json:"hosts"`
	Vars     map[string]interface{} `json:"vars,omitempty"`
	Children []string               `json:"children,omitempty"`
}

// Assert that we have the necessary environment variables set.
func assertEnvironSet() {
	// Assume first we have all flags, otherwise flip it later. This way we can print out all missing variables instead of
	// the user doing trial and error.
	hasVars := true
	for k, v := range environVars {
		_, err := os.LookupEnv(k)
		if err == false {
			hasVars = false
			fmt.Printf("error: %s environment variable is missing, %s\n", k, v)
		}
	}

	if !hasVars {
		os.Exit(1)
	}
}

// Create a new doClient to make all the API calls for us.
func createDoClient() doClient {
	return doClient{doApi, os.Getenv("DO_TOKEN")}
}

// A generic wrapper to make a GET call. Return []byte and let other functions handle what to do with it.
func (c doClient) doGet(endpoint string) []byte {
	// Make a new request because we need to add custom headers
	req, err := http.NewRequest("GET", c.api+endpoint, nil)
	if err != nil {
		fmt.Printf("error: failed to create request to %s, %s\n", endpoint, err)
		os.Exit(1)
	}

	// Add the necessary header
	req.Header.Add("Authorization", "Bearer "+c.token)

	// Create a new HTTP client and make the call
	hc := http.Client{}
	resp, err := hc.Do(req)
	if err != nil {
		fmt.Printf("error: failed to get %s, %s\n", req.URL, err)
		os.Exit(1)
	}

	// Get the byte array and return it
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		fmt.Printf("error: failed to read body, %s\n", err)
	}

	return b
}

// Generate the inventory structures
func (c doClient) createInventory() {
	// Get my droplet output first
	b := c.doGet("/droplets")

	// Convert it into the struct
	dResponse := dropletsResponse{}
	err := json.Unmarshal(b, &dResponse)
	if err != nil {
		fmt.Println("error: unmarshal failed, %s\n", err)
	}

	// Iterate every droplet
	for _, d := range dResponse.Droplets {
		// Group based on region
		if _, ok := ai.Groups[ansibleGroupPrefix+d.Region.Slug]; !ok {
			// Region does not exist, init the ansibleGroup with the first host
			ai.Groups[ansibleGroupPrefix+d.Region.Slug] = &ansibleGroup{
				Hosts: []string{d.Name},
			}
		} else {
			// Region exists, just append the host
			ai.Groups[ansibleGroupPrefix+d.Region.Slug].Hosts = append(ai.Groups[ansibleGroupPrefix+d.Region.Slug].Hosts, d.Name)
		}

		// Groups based on tags
		for _, t := range d.Tags {
			if _, ok := ai.Groups[ansibleGroupPrefix+t]; !ok {
				ai.Groups[ansibleGroupPrefix+t] = &ansibleGroup{
					Hosts: []string{d.Name},
				}
			} else {
				ai.Groups[ansibleGroupPrefix+t].Hosts = append(ai.Groups[ansibleGroupPrefix+t].Hosts, d.Name)
			}
		}

		// Setup hostvars for each droplet now
		if _, ok := ai.Meta.Hostvars[d.Name]; !ok {
			ai.Meta.Hostvars[d.Name] = make(map[string]interface{})
		}
		// Set up ansible_ssh per host
		for _, n := range d.Networks.V4 {
			// We want only the Public IPv4
			if n.Type == "public" {
				ai.Meta.Hostvars[d.Name].(map[string]interface{})["ansible_host"] = n.IpAddress
				break
			}
		}
	}
}

// Print the inventory out to stdout for ansible to suck up
func (a ansibleInventory) printInventory() {
	// A map to hold the final output
	printMap := make(map[string]interface{})

	// Iterate over groups first
	for k, v := range ai.Groups {
		printMap[k] = v
	}

	// Init the nested maps under _meta
	printMap["_meta"] = ai.Meta

	// Print it out in JSON
	jsonOut, err := json.Marshal(printMap)
	if err != nil {
		fmt.Printf("error: marshal failed, %s", err)
		os.Exit(1)
	}
	fmt.Printf("%s", jsonOut)
}

func main() {
	// Check our environment variables
	assertEnvironSet()

	// Initialize the maps inside of our global ansibleInventory
	ai.Groups = make(map[string]*ansibleGroup)
	ai.Meta.Hostvars = make(map[string]interface{})

	// Do work
	doClient := createDoClient()
	doClient.createInventory()
	ai.printInventory()
}
