country={{ .CountryCode}}
ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev
network={
    ssid="{{ .SSID}}"
    psk="{{ .Password }}"
    key_mgmt=WPA-PSK
}
