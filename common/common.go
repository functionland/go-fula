package common

const Version0 = "0"

// MemberStatus represents the approval status of a peer
type MemberStatus int

const (
	Unknown  MemberStatus = iota // iota provides automatic enumeration. Here, Pending = 0
	Pending                      // Pending = 1
	Approved                     // Approved = 2
)
