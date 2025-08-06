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
	PoolIds              string
	IsPeerIdMemberOfPool string
	GetMemberPeerIds     string
	RemoveMemberPeerId   string
}{
	Pools:                "0xced08b2d", // pools(uint32) - updated signature
	PoolIds:              "0x69883b4e", // poolIds(uint256) - index-based pool discovery
	IsPeerIdMemberOfPool: "0xb098a605", // isPeerIdMemberOfPool(uint32,bytes32) - corrected from block explorer
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

	// Correct ABI layout for pools(uint32) response:
	// Slot 0: creator (address)
	// Slot 1: id (uint32)
	// Slot 2: maxChallengeResponsePeriod (uint32)
	// Slot 3: memberCount (uint32)
	// Slot 4: maxMembers (uint32)
	// Slot 5: requiredTokens (uint256)
	// Slot 6: minPingTime (uint256)
	// Slot 7: name offset
	// Slot 8: region offset
	// Slot 9+: string data

	// Slot 0: creator address
	creatorHex := slots[0][24:64] // Last 40 hex chars = 20 bytes (address is right-aligned)
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

	// Slot 1: id (uint32)
	id, err := strconv.ParseUint(slots[1][56:64], 16, 32) // Last 8 hex chars for uint32
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool ID: %w", err)
	}
	pool.ID = uint32(id)

	// Slot 2: maxChallengeResponsePeriod (uint32)
	maxChallenge, err := strconv.ParseUint(slots[2][56:64], 16, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse maxChallengeResponsePeriod: %w", err)
	}
	pool.MaxChallengeResponsePeriod = uint32(maxChallenge)

	// Slot 3: memberCount (uint32)
	memberCount, err := strconv.ParseUint(slots[3][56:64], 16, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse memberCount: %w", err)
	}
	pool.MemberCount = uint32(memberCount)

	// Slot 4: maxMembers (uint32)
	maxMembers, err := strconv.ParseUint(slots[4][56:64], 16, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse maxMembers: %w", err)
	}
	pool.MaxMembers = uint32(maxMembers)

	// Slot 5: requiredTokens (uint256)
	requiredTokens := new(big.Int)
	requiredTokens.SetString(slots[5], 16)
	pool.RequiredTokens = requiredTokens

	// Slot 6: minPingTime (uint256)
	minPingTime := new(big.Int)
	minPingTime.SetString(slots[6], 16)
	pool.MinPingTime = minPingTime

	// Slot 7: offset to name string
	nameOffset, err := strconv.ParseUint(slots[7], 16, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse name offset: %w", err)
	}

	// Slot 8: offset to region string
	regionOffset, err := strconv.ParseUint(slots[8], 16, 64)
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
			// Only take the first nameLength*2 hex chars (each byte = 2 hex chars)
			if int(nameLength*2) <= len(nameHex) {
				nameHex = nameHex[:nameLength*2]
			}
			nameBytes, err := hex.DecodeString(nameHex)
			if err != nil {
				return nil, fmt.Errorf("failed to decode name: %w", err)
			}
			pool.Name = string(nameBytes)
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
			// Only take the first regionLength*2 hex chars (each byte = 2 hex chars)
			if int(regionLength*2) <= len(regionHex) {
				regionHex = regionHex[:regionLength*2]
			}
			regionBytes, err := hex.DecodeString(regionHex)
			if err != nil {
				return nil, fmt.Errorf("failed to decode region: %w", err)
			}
			pool.Region = string(regionBytes)
		}
	}

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

// EncodePoolIdsCall encodes the poolIds(uint256) method call for index-based pool discovery
func EncodePoolIdsCall(index uint32) string {
	return fmt.Sprintf("%s%064x", MethodSignatures.PoolIds, index)
}

// DecodePoolIdResponse decodes the response from poolIds(uint256) call
func DecodePoolIdResponse(result string) (uint32, error) {
	// Remove 0x prefix
	if strings.HasPrefix(result, "0x") {
		result = result[2:]
	}
	
	// Should be 64 hex characters (32 bytes)
	if len(result) != 64 {
		return 0, fmt.Errorf("invalid response length: expected 64 hex chars, got %d", len(result))
	}
	
	// Parse as uint64 first, then convert to uint32
	poolID, err := strconv.ParseUint(result, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse pool ID: %w", err)
	}
	
	return uint32(poolID), nil
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
