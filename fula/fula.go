package fula

import (
	libp2p "github.com/libp2p/go-libp2p"
	host "github.com/libp2p/go-libp2p-host"
)

type MobileLibp2p struct {
  node *host.Host
}

func StartLibp2p() bool {
  _, err := libp2p.New()
  
  if err != nil {
    return false
  }

  return true;
}