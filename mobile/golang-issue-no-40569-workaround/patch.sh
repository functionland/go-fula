#!/bin/bash
set -e
GOSRCDIR=$(dirname $(dirname $(which go)))/src
GOBIN=$(dirname $(dirname $(which go)))/bin/go
INTERFACE_SRC_FILE=$GOSRCDIR/net/interface_linux.go
NETLINK_SRC_FILE=$GOSRCDIR/syscall/netlink_linux.go
sudo cp $INTERFACE_SRC_FILE orig/interface_linux.go.bak
sudo cp $NETLINK_SRC_FILE orig/netlink_linux.go.bak
sudo cp $INTERFACE_SRC_FILE interface_linux.go
sudo cp $NETLINK_SRC_FILE netlink_linux.go
sudo git apply -C1 --recount netlink_linux.patch
sudo git apply -C1 --recount interface_linux.patch
sudo cp interface_linux.go $INTERFACE_SRC_FILE 
sudo cp netlink_linux.go $NETLINK_SRC_FILE 
sudo rm -rf interface_linux.go  netlink_linux.go
sudo $GOBIN install -a net
sudo $GOBIN install -a syscall