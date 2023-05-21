#!/bin/bash
set -e
GOSRCDIR=$(dirname $(dirname $(which go)))/src
GOBIN=$(dirname $(dirname $(which go)))/bin/go
INTERFACE_SRC_FILE=$GOSRCDIR/net/interface_linux.go
NETLINK_SRC_FILE=$GOSRCDIR/syscall/netlink_linux.go
sudo cp orig/interface_linux.go.bak $INTERFACE_SRC_FILE
sudo cp orig/netlink_linux.go.bak $NETLINK_SRC_FILE
sudo $GOBIN install -a net
sudo $GOBIN install -a syscall