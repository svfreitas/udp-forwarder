package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
)

func main() {

	filename := flag.String("f", "", "configuration file")
	flag.Parse()
	if *filename == "" {
		flag.Usage()
		fmt.Printf("Must provide configuration file: udp-forwarder -f <filename>")
		return
	}
	config, err := LoadConfiguration(*filename)

	if err != nil {
		log.Fatalf("Unable to load configuration, error :%s", err)
	}

	httpLogHandler := InitLogger(config.LogFileLocation, config.LogFileSize, config.LogFileMaxBackups)
	if slogger == nil {
		log.Fatalf("Unable to create logger")
	}
	defer slogger.Sync()

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/log/level", httpLogHandler)
		host := fmt.Sprintf("localhost:%d", config.LogLevelConfigPort)
		err := http.ListenAndServe(host, mux)
		if err != nil {
			slogger.Warnf("Unable to create log configuration http server on  %d", host)
		}
	}()

	sender, err := newRawSender(config.InterfaceName)
	if err != nil {
		log.Fatal(err)
	}
	defer sender.Close()

	addr := net.UDPAddr{
		Port: int(config.UdpPortReceiver),
		IP:   config.IpAddressReceiver,
	}
	listenStr := fmt.Sprintf("Listening on [%v]", addr)
	slogger.Infof(listenStr)
	fmt.Print(listenStr)

	conn, err := net.ListenUDP("udp4", &addr) // code does not block here
	if err != nil {
		slogger.Fatalf("Unable to listern on address [%v]: %s", addr, err)
	}
	defer conn.Close()

	var buf = make([]byte, 65527)
	for {
		rlen, remote, err := conn.ReadFromUDP(buf)

		if err != nil {
			slogger.Fatalf("Unable to read from connection]: %s", err)
		}
		slogger.Infof("Data read from [%v], length %d", remote, rlen)
		handlePacket2(sender, buf[:rlen], remote, config)
	}
}

func handlePacket2(sender *rawSender, payload []byte, remote *net.UDPAddr, config *Config) {
	var udpFrameOptions UdpFrameOptions

	udpFrameOptions.sourceIP = remote.IP
	udpFrameOptions.sourcePort = uint16(remote.Port)
	udpFrameOptions.payloadBytes = payload
	slogger.Debugf("udpFrameOptions.sourceIP = %v", udpFrameOptions.sourceIP)

	for _, destination := range config.Destinations {
		udpFrameOptions.destIP = destination.IpAddress
		udpFrameOptions.destPort = destination.Port

		slogger.Debugf("udpFrameOptions.destIP = %v", udpFrameOptions.destIP)
		slogger.Debugf("udpFrameOptions.destPort = %v", udpFrameOptions.destPort)

		slicePackets, err := createSerializedUDPPackets(udpFrameOptions)

		if err != nil {
			slogger.Errorf("Error serializing UDP packet to send to destination %s : %s", destination.IpAddress.String(), err)
			continue
		}
		slogger.Debugf("len(slicePackets) = %d", len(slicePackets))
		slogger.Debugf("Sending data to %s", destination.IpAddress.String())

		for i, packet := range slicePackets {
			slogger.Debugf("Sending packet[%d] = %v", i, packet)
			if err := sender.Send(destination.IpAddress, packet); err != nil {
				slogger.Errorf("Error Writing UDP data to destination %s : %s ", destination.IpAddress.String(), err)
			}
		}
	}
}
