# Go-Fula Comprehensive Test Plan

## Overview
This document outlines the comprehensive testing strategy for the go-fula library after the EVM blockchain migration from substrate to Base/Skale chains.

## Testing Strategy
- **Unit Tests**: Test individual functions and methods
- **Integration Tests**: Test module interactions
- **Edge Cases**: Test error conditions, timeouts, and boundary conditions
- **Production Scenarios**: Test real-world usage patterns

## Module Testing Plan

### 1. Blockchain Module (`blockchain/`)

#### Core Functionality Tests
- **Chain Configuration**
  - `GetChainConfigs()` returns correct Base and Skale configurations
  - Chain selection and fallback mechanisms
  - Invalid chain name handling

- **EVM Chain Calls**
  - `callEVMChain()` with valid/invalid parameters
  - `callEVMChainWithRetry()` retry logic and exponential backoff
  - JSON-RPC request/response handling
  - Network timeout and error scenarios

- **Pool Operations (EVM)**
  - `PoolJoin()` with EVM contract integration
  - `PoolLeave()` with `removeMemberPeerId` contract method
  - `HandleEVMPoolList()` pool discovery
  - `HandleIsMemberOfPool()` membership verification

- **Account Management**
  - `AccountExists()`, `AccountCreate()`, `AccountBalance()`
  - `AccountFund()` with proper amount handling
  - `AssetsBalance()` for different asset types

#### Edge Cases
- Network connectivity issues
- Invalid contract addresses
- Malformed JSON-RPC responses
- Chain synchronization failures
- Timeout scenarios

### 2. Mobile Module (`mobile/`)

#### Client Operations
- **Configuration**
  - `NewClient()` with various configurations
  - Blockchain endpoint validation
  - Pool name and chain selection

- **Data Operations**
  - `Put()` and `Get()` with different codecs
  - `Push()` and `Pull()` operations
  - `Store()` and `Load()` functionality

- **Blockchain Integration**
  - Mobile blockchain operations (AccountExists, AccountCreate, etc.)
  - Pool join/leave from mobile clients
  - Error handling for blockchain failures

#### Edge Cases
- Invalid multiaddr formats
- Network disconnections during operations
- Large data transfers
- Concurrent operations

### 3. Blox Module (`blox/`)

#### Core Functionality
- **Storage Operations**
  - `StoreCid()` with replication limits
  - `StoreManifest()` batch operations
  - IPFS integration and pinning

- **Exchange Operations**
  - `Push()` and `Pull()` between peers
  - Provider discovery via IPNI
  - Relay connectivity

- **Blockchain Integration**
  - Pool member management
  - Chain status monitoring
  - Account verification

#### Edge Cases
- Storage capacity limits
- Provider unavailability
- Network partitions
- Concurrent storage requests

### 4. Exchange Module (`exchange/`)

#### Protocol Operations
- **Data Exchange**
  - `Push()` and `Pull()` operations
  - `PullBlock()` single block retrieval
  - Authorization mechanisms

- **Provider Discovery**
  - IPNI provider finding
  - DHT operations
  - Relay usage

#### Edge Cases
- Provider timeouts
- Authorization failures
- Network congestion
- Large file transfers

### 5. WAP Module (`wap/`)

#### Server Operations
- **Pool Management**
  - Join/leave pool endpoints
  - Chain status reporting
  - Account management

- **WiFi Operations**
  - Network scanning and connection
  - Access point management

#### Edge Cases
- Invalid pool configurations
- Network connectivity issues
- Concurrent requests

## Test Implementation Priority

### Phase 1: Critical Path Tests
1. Blockchain EVM integration tests
2. Pool join/leave operations
3. Basic mobile client operations
4. Core blox storage functionality

### Phase 2: Integration Tests
1. Mobile-to-blox communication
2. Multi-chain operations
3. Provider discovery and exchange
4. WAP server endpoints

### Phase 3: Edge Cases and Performance
1. Error handling scenarios
2. Network failure recovery
3. Concurrent operations
4. Load testing

## Test Data and Mocking

### Mock Services
- Mock EVM RPC endpoints for Base/Skale
- Mock IPFS nodes
- Mock relay servers
- Mock blockchain contracts

### Test Data
- Valid/invalid peer IDs
- Sample CIDs and IPLD nodes
- Chain configuration variations
- Network addresses and multiaddrs

## Success Criteria
- All unit tests pass
- Integration tests cover main workflows
- Edge cases are handled gracefully
- No blocking operations cause application failures
- Backward compatibility maintained where applicable
