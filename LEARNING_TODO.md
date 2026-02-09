# Learning TODO: Merkle Patricia Trie in Go

This is your structured learning path to master the Merkle Patricia Trie and Go programming.
Each phase builds upon the previous one. Complete the tasks in order!

Reference repository: https://github.com/zhangchiqing/merkle-patricia-trie

---

## Phase 1: Understand the Data Structure Concepts

Read and understand the theory before coding.

- [ ] **Task 1.1**: Understand **Hex-Prefix Encoding** (Compact Encoding)
  - How nibbles (half-bytes) are used as path elements
  - How prefixes distinguish leaf nodes from extension nodes
  - How odd/even path lengths are handled
- [ ] **Task 1.2**: Understand **RLP (Recursive Length Prefix) Encoding**
  - This is Ethereum's serialization format
  - Know what it does conceptually (you'll use a library for it)

---

## Phase 2: Implement Nibble Utilities

Start coding! Nibbles are the foundation of path encoding in the trie.

> **Reference file**: `nibbles.go` in the reference repo

- [ ] **Task 2.1**: Define the `Nibble` type
  - A nibble is a 4-bit value (0–15), represented as a `byte`
  - Create `type Nibble byte`
  - Implement `IsNibble(byte) bool` validation function
- [ ] **Task 2.2**: Implement byte-to-nibble conversion functions
  - `FromByte(b byte) []Nibble` — split one byte into two nibbles
  - `FromBytes(bs []byte) []Nibble` — convert a byte slice to nibbles
  - `FromString(s string) []Nibble` — convenience wrapper
- [ ] **Task 2.3**: Implement nibble-to-byte conversion
  - `ToBytes(ns []Nibble) []byte` — combine pairs of nibbles back into bytes
- [ ] **Task 2.4**: Implement Hex-Prefix encoding
  - `ToPrefixed(ns []Nibble, isLeafNode bool) []Nibble`
  - This adds a prefix to indicate leaf vs extension and handles odd/even lengths
- [ ] **Task 2.5**: Implement prefix matching
  - `PrefixMatchedLen(node1, node2 []Nibble) int`
  - Returns the number of matching nibbles from the start
- [ ] **Task 2.6**: Write tests for all nibble functions
  - Test edge cases: empty slices, single byte, odd/even lengths

---

## Phase 3: Implement the Node Types

Build the four node types that make up the trie.

### 3A: Empty Node

> **Reference file**: `empty.go`

- [ ] **Task 3A.1**: Define empty node constants
  - `EmptyNodeRaw` — empty byte slice
  - `EmptyNodeHash` — the Keccak256 hash of RLP-encoded empty string
  - Implement `IsEmptyNode(node Node) bool`

### 3B: Node Interface

> **Reference file**: `nodes.go`

- [ ] **Task 3B.1**: Define the `Node` interface
  - Methods: `Hash() []byte` and `Raw() []interface{}`
  - Implement `Hash(node Node) []byte` helper that handles empty nodes
  - Implement `Serialize(node Node) []byte` that RLP-encodes a node

### 3C: Leaf Node

> **Reference file**: `leaf.go`

- [ ] **Task 3C.1**: Define the `LeafNode` struct
  - Fields: `Path []Nibble` and `Value []byte`
  - Implement constructor functions
- [ ] **Task 3C.2**: Implement `LeafNode` methods
  - `Hash()` — Keccak256 of serialized node
  - `Raw()` — returns `[prefixed_path, value]` for RLP encoding
  - `Serialize()` — RLP encode the raw representation
- [ ] **Task 3C.3**: Write tests for LeafNode

### 3D: Branch Node

> **Reference file**: `branch.go`

- [ ] **Task 3D.1**: Define the `BranchNode` struct
  - Fields: `Branches [16]Node` and `Value []byte`
  - A branch has 16 slots (one per nibble 0–f) plus an optional value
- [ ] **Task 3D.2**: Implement `BranchNode` methods
  - `SetBranch(nibble, node)`, `RemoveBranch(nibble)`
  - `SetValue(value)`, `RemoveValue()`, `HasValue() bool`
  - `Hash()`, `Raw()`, `Serialize()`
  - Note: `Raw()` returns 17 elements (16 branches + value)
- [ ] **Task 3D.3**: Write tests for BranchNode

### 3E: Extension Node

> **Reference file**: `extension.go`

- [ ] **Task 3E.1**: Define the `ExtensionNode` struct
  - Fields: `Path []Nibble` and `Next Node`
  - An extension node compresses a shared path prefix
- [ ] **Task 3E.2**: Implement `ExtensionNode` methods
  - `Hash()`, `Raw()`, `Serialize()`
  - Note: `Raw()` returns `[prefixed_path, next_hash_or_raw]`
- [ ] **Task 3E.3**: Write tests for ExtensionNode

---

## Phase 4: Implement the Trie Operations

This is the core of the project — putting it all together.

> **Reference file**: `trie.go`

- [ ] **Task 4.1**: Define the `Trie` struct and constructor
  - `type Trie struct { root Node }`
  - `NewTrie() *Trie`
  - `Hash() []byte` — returns the hash of the root node
- [ ] **Task 4.2**: Implement `Get(key []byte) ([]byte, bool)`
  - Traverse the trie from root following the nibble path
  - Handle all node types: Empty → not found, Leaf → check path match, Branch → follow nibble, Extension → match prefix then continue
- [ ] **Task 4.3**: Implement `Put(key []byte, value []byte)` — Empty & Leaf cases
  - When stopped at EmptyNode → create a new LeafNode
  - When stopped at LeafNode → create Branch + Extension as needed
  - This is the most complex part — study the reference implementation carefully!
- [ ] **Task 4.4**: Implement `Put` — Branch case
  - When stopped at BranchNode → follow the nibble or set value
- [ ] **Task 4.5**: Implement `Put` — Extension case
  - When stopped at ExtensionNode → split the extension if needed
  - Handle partial matches, full matches, and zero matches
- [ ] **Task 4.6**: Write comprehensive tests for Get and Put
  - Test: get from empty trie, put and get, update existing value
  - Test: data integrity — same key-value pairs produce same root hash
  - Test: insertion order doesn't matter
  - Test: various tree structures (leaf splitting, extension splitting)

---

## Phase 5: Implement Hashing and Cryptography

> **Reference file**: `crypto.go`

- [ ] **Task 5.1**: Implement or integrate Keccak256 hashing
  - Use `golang.org/x/crypto/sha3` package
  - Or use `github.com/ethereum/go-ethereum/crypto`
- [ ] **Task 5.2**: Verify hash correctness
  - Compare your trie's root hash with the reference implementation for the same inputs
  - The hash must be deterministic — same data always produces the same hash

---

## Phase 6: Implement Merkle Proofs (Advanced)

> **Reference file**: `proof.go`

- [ ] **Task 6.1**: Implement `ProofDB` — a simple key-value store for proof nodes
  - Methods: `Put`, `Get`, `Delete`, `Has`
- [ ] **Task 6.2**: Implement `Prove(key []byte) (Proof, bool)`
  - Walk the trie, collecting each node's hash and serialization along the path
- [ ] **Task 6.3**: Implement proof verification
  - Given a root hash, key, and proof, verify the key exists in the trie
- [ ] **Task 6.4**: Write tests for Merkle proofs

---

## Phase 7: Ethereum Applications (Bonus)

These are optional advanced tasks that connect the trie to real Ethereum use cases.

> **Reference files**: `transaction.go`, `storage_proof.go`, `erc20_proof.go`

- [ ] **Task 7.1**: Study how Ethereum transactions are stored in a trie
- [ ] **Task 7.2**: Study Ethereum account state proofs (EIP-1186)
- [ ] **Task 7.3**: Study ERC20 storage proofs — proving token balances

---

## Progress Tracker

| Phase | Topic | Status |
|-------|-------|--------|
| 1 | Data Structure Concepts | ⬜ Not Started |
| 2 | Nibble Utilities | ⬜ Not Started |
| 3 | Node Types | ⬜ Not Started |
| 4 | Trie Operations | ⬜ Not Started |
| 5 | Hashing & Crypto | ⬜ Not Started |
| 6 | Merkle Proofs | ⬜ Not Started |
| 7 | Ethereum Applications | ⬜ Not Started |

---

**Tips:**
- Take your time with each phase. Understanding > speed.
- Run `go test ./...` frequently to verify your implementation.
- When stuck, read the corresponding file in the reference repo and study the test cases.
- Ask AI (your teacher) for hints before looking at full solutions!
