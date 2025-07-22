package abi

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// ContractError represents a custom error from the contract
type ContractError struct {
	Code    string
	Message string
}

func (e ContractError) Error() string {
	return fmt.Sprintf("contract error %s: %s", e.Code, e.Message)
}

// Contract error signatures (4-byte selectors)
var ContractErrorSignatures = map[string]ContractError{
	"0x12345678": {Code: "PNF", Message: "PoolNotFound"},
	"0x87654321": {Code: "AIP", Message: "AlreadyInPool"},
	"0x11111111": {Code: "ARQ", Message: "AlreadyRequested"},
	"0x22222222": {Code: "PNF2", Message: "PeerNotFound"},
	"0x33333333": {Code: "CR", Message: "CapacityReached"},
	"0x44444444": {Code: "UF", Message: "UserFlagged"},
	"0x55555555": {Code: "NM", Message: "NotMember"},
	"0x66666666": {Code: "AV", Message: "AlreadyVoted"},
	"0x77777777": {Code: "NAR", Message: "NoActiveRequest"},
	"0x88888888": {Code: "OCA", Message: "OnlyCreatorOrAdmin"},
	"0x99999999": {Code: "PNE", Message: "PoolNotEmpty"},
	"0xAAAAAAAA": {Code: "PRE", Message: "PendingRequestsExist"},
	"0xBBBBBBBB": {Code: "ITA", Message: "InvalidTokenAmount"},
	"0xCCCCCCCC": {Code: "IA", Message: "InsufficientAllowance"},
}

// ParseContractError attempts to parse a contract error from the response
func ParseContractError(data string) error {
	if !strings.HasPrefix(data, "0x") || len(data) < 10 {
		return nil
	}

	// Extract the first 4 bytes (8 hex chars + 0x prefix)
	errorSig := data[:10]

	if contractErr, exists := ContractErrorSignatures[errorSig]; exists {
		return contractErr
	}

	return nil
}

// Pool represents the Pool struct from the contract
type Pool struct {
	Creator                    string   `json:"creator"`
	ID                         uint32   `json:"id"`
	MaxChallengeResponsePeriod uint32   `json:"maxChallengeResponsePeriod"`
	MemberCount                uint32   `json:"memberCount"`
	MaxMembers                 uint32   `json:"maxMembers"`
	RequiredTokens             *big.Int `json:"requiredTokens"`
	MinPingTime                *big.Int `json:"minPingTime"`
	Name                       string   `json:"name"`
	Region                     string   `json:"region"`
}

// IsMemberOfPoolResult represents the result of isPeerIdMemberOfPool call
type IsMemberOfPoolResult struct {
	IsMember      bool   `json:"isMember"`
	MemberAddress string `json:"memberAddress"`
}

// GetMemberPeerIdsResult represents the result of getMemberPeerIds call
type GetMemberPeerIdsResult struct {
	PeerIds []string `json:"peerIds"` // Array of bytes32 peer IDs
}

// MethodSignatures contains the 4-byte method signatures
var MethodSignatures = struct {
	Pools                string
	IsPeerIdMemberOfPool string
	GetMemberPeerIds     string
	RemoveMemberPeerId   string
}{
	Pools:                "0xced08b2d", // pools(uint32) - updated signature
	IsPeerIdMemberOfPool: "0x95c5eb1a", // isPeerIdMemberOfPool(uint32,bytes32)
	GetMemberPeerIds:     "0x31db3ae8", // getMemberPeerIds(uint32,address)
	RemoveMemberPeerId:   "0x12345678", // removeMemberPeerId(uint32,bytes32) - TODO: Update with actual signature
}

// DecodePoolsResult decodes the result from pools(uint32) contract call
func DecodePoolsResult(data string) (*Pool, error) {
	// Remove 0x prefix
	if strings.HasPrefix(data, "0x") {
		data = data[2:]
	}

	// Check if we have enough data (minimum 9 * 32 bytes = 288 hex chars)
	if len(data) < 288 {
		return nil, fmt.Errorf("insufficient data length: %d, expected at least 288", len(data))
	}

	// Decode each 32-byte slot
	slots := make([]string, 0)
	for i := 0; i < len(data); i += 64 {
		if i+64 <= len(data) {
			slots = append(slots, data[i:i+64])
		}
	}

	if len(slots) < 9 {
		return nil, fmt.Errorf("insufficient slots: %d, expected at least 9", len(slots))
	}

	pool := &Pool{}

	// According to the struct layout:
	// Slot 0: creator (20 bytes) + id (4 bytes) + maxChallengeResponsePeriod (4 bytes) + memberCount (4 bytes)
	slot0 := slots[0]

	// Creator address (first 20 bytes, left-padded to 32 bytes)
	creatorHex := slot0[24:64] // Last 40 hex chars = 20 bytes
	pool.Creator = "0x" + creatorHex

	// Check if creator is zero address (pool doesn't exist)
	isZeroAddress := true
	for _, char := range creatorHex {
		if char != '0' {
			isZeroAddress = false
			break
		}
	}
	if isZeroAddress {
		return nil, fmt.Errorf("pool not found (zero creator address)")
	}

	// Parse packed fields from slot0 (before the address)
	// id (4 bytes), maxChallengeResponsePeriod (4 bytes), memberCount (4 bytes)
	packedFields := slot0[0:24] // First 24 hex chars = 12 bytes

	// Parse ID (first 4 bytes)
	if len(packedFields) >= 8 {
		idHex := packedFields[0:8]
		id, err := strconv.ParseUint(idHex, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pool ID: %w", err)
		}
		pool.ID = uint32(id)
	}

	// Parse maxChallengeResponsePeriod (next 4 bytes)
	if len(packedFields) >= 16 {
		maxChallengeHex := packedFields[8:16]
		maxChallenge, err := strconv.ParseUint(maxChallengeHex, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse maxChallengeResponsePeriod: %w", err)
		}
		pool.MaxChallengeResponsePeriod = uint32(maxChallenge)
	}

	// Parse memberCount (next 4 bytes)
	if len(packedFields) >= 24 {
		memberCountHex := packedFields[16:24]
		memberCount, err := strconv.ParseUint(memberCountHex, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse memberCount: %w", err)
		}
		pool.MemberCount = uint32(memberCount)
	}

	// Slot 1: maxMembers (4 bytes) + padding
	slot1 := slots[1]

	// Parse maxMembers from first 8 hex chars (4 bytes) of slot1
	maxMembersHex := slot1[56:64] // Last 8 hex chars for right-aligned uint32
	maxMembers, err := strconv.ParseUint(maxMembersHex, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse maxMembers: %w", err)
	}
	pool.MaxMembers = uint32(maxMembers)

	// Slot 2: requiredTokens (uint256)
	slot2 := slots[2]
	requiredTokens := new(big.Int)
	requiredTokens.SetString(slot2, 16)
	pool.RequiredTokens = requiredTokens

	// Slot 3: minPingTime (uint256)
	slot3 := slots[3]
	minPingTime := new(big.Int)
	minPingTime.SetString(slot3, 16)
	pool.MinPingTime = minPingTime

	// Slot 4: offset to name string
	slot4 := slots[4]
	nameOffset, err := strconv.ParseUint(slot4, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse name offset: %w", err)
	}

	// Slot 5: offset to region string
	slot5 := slots[5]
	regionOffset, err := strconv.ParseUint(slot5, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse region offset: %w", err)
	}

	// Parse name string
	nameSlotIndex := int(nameOffset / 32)
	if nameSlotIndex < len(slots) {
		nameLength, err := strconv.ParseUint(slots[nameSlotIndex], 16, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse name length: %w", err)
		}

		if nameLength > 0 && nameSlotIndex+1 < len(slots) {
			nameHex := slots[nameSlotIndex+1]
			nameBytes, err := hex.DecodeString(nameHex)
			if err != nil {
				return nil, fmt.Errorf("failed to decode name: %w", err)
			}
			// Trim null bytes
			pool.Name = strings.TrimRight(string(nameBytes), "\x00")
		}
	}

	// Parse region string
	regionSlotIndex := int(regionOffset / 32)
	if regionSlotIndex < len(slots) {
		regionLength, err := strconv.ParseUint(slots[regionSlotIndex], 16, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse region length: %w", err)
		}

		if regionLength > 0 && regionSlotIndex+1 < len(slots) {
			regionHex := slots[regionSlotIndex+1]
			regionBytes, err := hex.DecodeString(regionHex)
			if err != nil {
				return nil, fmt.Errorf("failed to decode region: %w", err)
			}
			// Trim null bytes
			pool.Region = strings.TrimRight(string(regionBytes), "\x00")
		}
	}

	// Parse remaining fields from the packed slot0
	// Extract id, maxChallengeResponsePeriod, memberCount from slot0
	// These are packed in the first part of slot0 before the address

	// For now, we'll extract what we can from the available data
	// In a full implementation, you'd need to properly parse the packed struct

	return pool, nil
}

// DecodeIsMemberOfPoolResult decodes the result from isPeerIdMemberOfPool(uint32,bytes32) contract call
func DecodeIsMemberOfPoolResult(data string) (*IsMemberOfPoolResult, error) {
	// Remove 0x prefix
	if strings.HasPrefix(data, "0x") {
		data = data[2:]
	}

	// Check if we have enough data (2 * 32 bytes = 128 hex chars)
	if len(data) < 128 {
		return nil, fmt.Errorf("insufficient data length: %d, expected 128", len(data))
	}

	result := &IsMemberOfPoolResult{}

	// First 32 bytes: isMember (bool)
	isMemberHex := data[0:64]
	isMember := false
	for _, char := range isMemberHex {
		if char != '0' {
			isMember = true
			break
		}
	}
	result.IsMember = isMember

	// Next 32 bytes: memberAddress (address, right-aligned)
	memberAddressHex := data[64:128]
	// Extract the last 20 bytes (40 hex chars) for the address
	result.MemberAddress = "0x" + memberAddressHex[24:64]

	return result, nil
}

// DecodeGetMemberPeerIdsResult decodes the result from getMemberPeerIds(uint32,address) contract call
func DecodeGetMemberPeerIdsResult(data string) (*GetMemberPeerIdsResult, error) {
	// Remove 0x prefix
	if strings.HasPrefix(data, "0x") {
		data = data[2:]
	}

	// Check if we have enough data (minimum 64 hex chars for offset + length)
	if len(data) < 64 {
		return &GetMemberPeerIdsResult{PeerIds: []string{}}, nil
	}

	// First 32 bytes: offset to array data
	offsetHex := data[0:64]
	offset, err := strconv.ParseUint(offsetHex, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse array offset: %w", err)
	}

	// Calculate the position in hex chars (each byte = 2 hex chars)
	arrayStartPos := int(offset * 2)

	if arrayStartPos >= len(data) {
		return &GetMemberPeerIdsResult{PeerIds: []string{}}, nil
	}

	// Array length (32 bytes at the offset position)
	if arrayStartPos+64 > len(data) {
		return &GetMemberPeerIdsResult{PeerIds: []string{}}, nil
	}

	arrayLengthHex := data[arrayStartPos : arrayStartPos+64]
	arrayLength, err := strconv.ParseUint(arrayLengthHex, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse array length: %w", err)
	}

	result := &GetMemberPeerIdsResult{PeerIds: make([]string, 0, arrayLength)}

	// Parse each bytes32 element (32 bytes = 64 hex chars each)
	dataStartPos := arrayStartPos + 64
	for i := uint64(0); i < arrayLength; i++ {
		elementStartPos := dataStartPos + int(i*64)
		elementEndPos := elementStartPos + 64

		if elementEndPos > len(data) {
			break // Not enough data for this element
		}

		peerIdHex := data[elementStartPos:elementEndPos]
		result.PeerIds = append(result.PeerIds, "0x"+peerIdHex)
	}

	return result, nil
}

// EncodePoolsCall encodes the pools(uint32) method call
func EncodePoolsCall(poolID uint32) string {
	return fmt.Sprintf("%s%064x", MethodSignatures.Pools, poolID)
}

// EncodeIsPeerIdMemberOfPoolCall encodes the isPeerIdMemberOfPool(uint32,bytes32) method call
func EncodeIsPeerIdMemberOfPoolCall(poolID uint32, peerIDBytes32 string) string {
	// Remove 0x prefix from peerIDBytes32 if present
	if strings.HasPrefix(peerIDBytes32, "0x") {
		peerIDBytes32 = peerIDBytes32[2:]
	}

	return fmt.Sprintf("%s%064x%s", MethodSignatures.IsPeerIdMemberOfPool, poolID, peerIDBytes32)
}

// EncodeGetMemberPeerIdsCall encodes the getMemberPeerIds(uint32,address) method call
func EncodeGetMemberPeerIdsCall(poolID uint32, memberAddress string) string {
	// Remove 0x prefix from memberAddress if present
	if strings.HasPrefix(memberAddress, "0x") {
		memberAddress = memberAddress[2:]
	}

	// Pad address to 32 bytes (64 hex chars)
	paddedAddress := fmt.Sprintf("%064s", memberAddress)

	return fmt.Sprintf("%s%064x%s", MethodSignatures.GetMemberPeerIds, poolID, paddedAddress)
}

// EncodeRemoveMemberPeerIdCall encodes the removeMemberPeerId(uint32,bytes32) method call
func EncodeRemoveMemberPeerIdCall(poolID uint32, peerIDBytes32 string) string {
	// Remove 0x prefix from peerIDBytes32 if present
	if strings.HasPrefix(peerIDBytes32, "0x") {
		peerIDBytes32 = peerIDBytes32[2:]
	}

	return fmt.Sprintf("%s%064x%s", MethodSignatures.RemoveMemberPeerId, poolID, peerIDBytes32)
}
