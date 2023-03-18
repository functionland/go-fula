interface={{ .WifiInterface }}      # Use the require wireless interface - usually wlan0
  dhcp-range={{ .SubnetRangeStart }},{{ .SubnetRangeEnd }},{{ .Netmask }},24h
