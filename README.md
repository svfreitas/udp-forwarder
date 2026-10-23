# udp-forwarder

Simple udp packet forwarder preserving (spoofing) the original IP that sent the packet

Packets are sent through a raw IPv4 socket, so routing and ARP resolution are done by the kernel.
Linux only, IPv4 only, requires root or `CAP_NET_RAW`. No libpcap needed.

Configuration file example:

```json
{
    "interface_name" : "eth0",
    "ip_address_receiver" : "192.168.1.32:1162",
    "log_level_config_port" : 8080,
    "destinations" : [
        "192.168.1.31:2123"
    ]
}
```
