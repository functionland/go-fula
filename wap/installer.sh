#! /bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null  && pwd )

# Update apt get
sudo apt-get -y update
sudo apt --fix-broken -y install
sudo apt-get -y upgrade

# Remove preexistent dependencies
sudo apt-get --purge remove -y node
sudo apt-get --purge remove -y nodejs

# Install dependencies
sudo apt-get install -y nodejs
sudo apt-get install -y git
sudo apt-get install -y build-essential
sudo apt-get install -y libudev-dev
sudo apt-get install -y hostapd
sudo apt-get install -y dnsmasq
sudo apt-get install -y iw
sudo apt-get install -y npm

# Install pm2
sudo /usr/bin/npm install -g pm2
sudo /usr/bin/npm install -g node-gyp

# Activate rf interfaces
sudo rfkill unblock wifi
sudo rfkill unblock all

# Remove wpa_supplicant
sudo bash -c 'echo "" > /etc/wpa_supplicant/wpa_supplicant.conf'

# Remove ssh
sudo rm -rf /home/$SUDO_USER/.ssh

# Enable dhcpdcd
sudo systemctl enable systemd-networkd

# Setup project
cd $SCRIPT_DIR
go build -o wap main.go
chmod +x wap

# Start pm2
sudo pm2 start ./wap --name=Box
sudo pm2 startup
sudo pm2 save

# Reboot
sudo reboot
