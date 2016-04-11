package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/lair-framework/api-server/client"
	"github.com/lair-framework/api-server/lib/ip"
	"github.com/lair-framework/go-lair"
	"github.com/tomsteele/cookiescan"
)

const (
	version = "3.0.0"
	tool    = "cookiescan"
	usage   = `
Parses and imports a cookiescan JSON file into a lair project.

Usage:
  drone-cookiescan [options] <id> <filename>
  export LAIR_ID=<id>; drone-cookiescan [options] <filename>
Options:
  -v              show version and exit
  -h              show usage and exit
  -k              allow insecure SSL connections
  -force-ports    disable data protection in the API server for excessive ports
  -tags           a comma separated list of tags to add to every host that is imported
`
)

func main() {
	showVersion := flag.Bool("v", false, "")
	insecureSSL := flag.Bool("k", false, "")
	forcePorts := flag.Bool("force-ports", false, "")
	tags := flag.String("tags", "", "")
	flag.Usage = func() {
		fmt.Println(usage)
	}
	flag.Parse()
	if *showVersion {
		log.Println(version)
		os.Exit(0)
	}
	lairURL := os.Getenv("LAIR_API_SERVER")
	if lairURL == "" {
		log.Fatal("Fatal: Missing LAIR_API_SERVER environment variable")
	}
	lairPID := os.Getenv("LAIR_ID")
	if lairPID == "" {
		log.Fatal("Fatal: Missing LAIR_ID")
	}
	var filename string
	switch len(flag.Args()) {
	case 2:
		lairPID = flag.Arg(0)
		filename = flag.Arg(1)
	case 1:
		filename = flag.Arg(0)
	default:
		log.Fatal("Fatal: Missing required argument")
	}
	u, err := url.Parse(lairURL)
	if err != nil {
		log.Fatalf("Fatal: Error parsing LAIR_API_SERVER URL. Error %s", err.Error())
	}
	if u.User == nil {
		log.Fatal("Fatal: Missing username and/or password")
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	if user == "" || pass == "" {
		log.Fatal("Fatal: Missing username and/or password")
	}
	c, err := client.New(&client.COptions{
		User:               user,
		Password:           pass,
		Host:               u.Host,
		Scheme:             u.Scheme,
		InsecureSkipVerify: *insecureSSL,
	})
	if err != nil {
		log.Fatalf("Fatal: Error setting up client: Error %s", err.Error())
	}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Fatal: Could not open file. Error %s", err.Error())
	}
	hostTags := []string{}
	if *tags != "" {
		hostTags = strings.Split(*tags, ",")
	}
	l := lair.Project{
		ID:   lairPID,
		Tool: tool,
		Commands: []lair.Command{lair.Command{
			Tool:    tool,
			Command: "",
		}},
	}
	results := []cookiescan.Result{}
	if err := json.Unmarshal(data, &results); err != nil {
		log.Fatalf("Fatal: Could not parse JSON. Error %s", err.Error())
	}

	for _, r := range results {
		ipaddr := net.ParseIP(r.Host)
		host := lair.Host{
			IPv4:         ipaddr.To4().String(),
			LongIPv4Addr: ip.IpToInt(ipaddr.To4()),
			Tags:         hostTags,
		}
		for _, p := range r.Services {
			host.Services = append(host.Services, lair.Service{
				Port:     p.Port,
				Service:  p.Service,
				Product:  "unknown",
				Protocol: "tcp",
			})
		}
		l.Hosts = append(l.Hosts, host)
	}

	res, err := c.ImportProject(&client.DOptions{ForcePorts: *forcePorts}, &l)
	if err != nil {
		log.Fatalf("Fatal: Unable to import project. Error %s", err)
	}
	defer res.Body.Close()
	droneRes := &client.Response{}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("Fatal: Error %s", err.Error())
	}
	if err := json.Unmarshal(body, droneRes); err != nil {
		log.Fatalf("Fatal: Could not unmarshal JSON. Error %s", err.Error())
	}
	if droneRes.Status == "Error" {
		log.Fatalf("Fatal: Import failed. Error %s", droneRes.Message)
	}
	log.Println("Success: Operation completed successfully")
}
