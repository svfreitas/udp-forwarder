package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

type rawConfig struct {
	InterfaceName      string   `json:"interface_name"`
	LogFileLocation    string   `json:"log_file_location"`
	LogFileSize        int      `json:"log_file_size"`
	LogFileMaxBackups  int      `json:"log_file_max_backups"`
	LogLevelConfigPort int      `json:"log_level_config_port"`
	IpAddressReceiver  string   `json:"ip_address_receiver"`
	MaxPacketSize      int      `json:"max_packet_size"`
	Destinations       []string `json:"destinations"`
}

type Config struct {
	InterfaceName      string
	LogLevelConfigPort int
	LogFileLocation    string
	LogFileSize        int
	LogFileMaxBackups  int
	IpAddressReceiver  net.IP
	UdpPortReceiver    uint16
	MaxPacketSize      int
	Destinations       []Destination
}

type Destination struct {
	IpAddress net.IP
	Port      uint16
}

func LoadConfiguration(filename string) (*Config, error) {

	var config Config
	var rawConfig rawConfig
	var err error

	content, err := os.ReadFile(filename)

	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(content), &rawConfig)

	if err != nil {
		log.Fatalf("Configuration file problem : %s", err)
	}

	config.LogLevelConfigPort = rawConfig.LogLevelConfigPort
	if config.LogLevelConfigPort <= 0 {
		log.Fatal("log_level_config_port bad format")
	}

	config.LogFileLocation = rawConfig.LogFileLocation
	if config.LogFileLocation == "" {
		config.LogFileLocation = "./"
	} else if strings.HasSuffix(config.LogFileLocation, "/") == false {
		config.LogFileLocation += "/"
	}

	config.LogFileMaxBackups = rawConfig.LogFileMaxBackups
	if config.LogFileMaxBackups == 0 {
		config.LogFileMaxBackups = 3
	}

	config.LogFileSize = rawConfig.LogFileSize
	if config.LogFileSize == 0 {
		config.LogFileSize = 10
	}

	config.InterfaceName = rawConfig.InterfaceName
	if config.InterfaceName == "" {
		log.Fatal("interface_name missing")
	}

	host, port, err := net.SplitHostPort(rawConfig.IpAddressReceiver)
	if err != nil {
		log.Fatal("ip_address_receiver bad format")
	}

	config.IpAddressReceiver = net.ParseIP(host)
	if config.IpAddressReceiver == nil || config.IpAddressReceiver.To4() == nil {
		log.Fatal("ip_address_receiver bad format for host, must be an IPv4 address")
	}

	if _, err := net.InterfaceByName(config.InterfaceName); err != nil {
		log.Fatalf("interface_name problem: %s", err)
	}

	portUint, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		log.Fatal("Port bad format")
	}
	config.UdpPortReceiver = uint16(portUint)

	config.MaxPacketSize = rawConfig.MaxPacketSize

	for _, rawDest := range rawConfig.Destinations {
		var dest Destination
		host, port, err := net.SplitHostPort(rawDest)
		if err != nil {
			log.Fatalf("Host Port bad format for Destination: %s", rawDest)
		}

		dest.IpAddress = net.ParseIP(host)
		if dest.IpAddress == nil || dest.IpAddress.To4() == nil {
			log.Fatalf("IpAddressDestination bad format for Destination, must be an IPv4 address: %s", rawDest)
		}

		uPort, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			log.Fatalf("Port bad format for Destination: %s", rawDest)
		}
		dest.Port = uint16(uPort)

		config.Destinations = append(config.Destinations, dest)
	}

	return &config, nil

}
