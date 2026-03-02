package solana

import (
	"testing"
	"time"

	solanago "github.com/gagliardetto/solana-go"
)

// --- Memo instruction tests ---

func TestBuildSimpleMemoInstruction(t *testing.T) {
	data := []byte("hello ghostlink")
	inst := BuildSimpleMemoInstruction(data)

	// Verify the program ID is the Memo Program.
	if !inst.ProgramID().Equals(MemoProgramID) {
		t.Errorf("expected program ID %s, got %s", MemoProgramID, inst.ProgramID())
	}

	// Verify the instruction data matches what was provided.
	instData, err := inst.Data()
	if err != nil {
		t.Fatalf("unexpected error getting instruction data: %v", err)
	}
	if string(instData) != string(data) {
		t.Errorf("expected data %q, got %q", data, instData)
	}

	// Verify no accounts are attached.
	accounts := inst.Accounts()
	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(accounts))
	}
}

func TestBuildMemoInstruction(t *testing.T) {
	sender := solanago.NewWallet().PublicKey()
	data := []byte("encrypted message payload")

	inst := BuildMemoInstruction(data, sender)

	// Verify program ID.
	if !inst.ProgramID().Equals(MemoProgramID) {
		t.Errorf("expected program ID %s, got %s", MemoProgramID, inst.ProgramID())
	}

	// Verify instruction data.
	instData, err := inst.Data()
	if err != nil {
		t.Fatalf("unexpected error getting instruction data: %v", err)
	}
	if string(instData) != string(data) {
		t.Errorf("expected data %q, got %q", data, instData)
	}

	// Verify accounts: only the sender (signer+writable).
	accounts := inst.Accounts()
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}

	if !accounts[0].PublicKey.Equals(sender) {
		t.Errorf("expected sender account %s, got %s", sender, accounts[0].PublicKey)
	}
	if !accounts[0].IsSigner {
		t.Error("expected sender to be a signer")
	}
	if !accounts[0].IsWritable {
		t.Error("expected sender to be writable")
	}
}

func TestBuildTransferInstruction(t *testing.T) {
	sender := solanago.NewWallet().PublicKey()
	recipient := solanago.NewWallet().PublicKey()

	inst := BuildTransferInstruction(sender, recipient)

	if !inst.ProgramID().Equals(SystemProgramID) {
		t.Errorf("expected System Program ID, got %s", inst.ProgramID())
	}

	accounts := inst.Accounts()
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}

	if !accounts[0].PublicKey.Equals(sender) {
		t.Errorf("expected sender %s, got %s", sender, accounts[0].PublicKey)
	}
	if !accounts[0].IsSigner {
		t.Error("expected sender to be signer")
	}

	if !accounts[1].PublicKey.Equals(recipient) {
		t.Errorf("expected recipient %s, got %s", recipient, accounts[1].PublicKey)
	}
}

func TestBuildMemoInstructionSameSenderRecipient(t *testing.T) {
	wallet := solanago.NewWallet().PublicKey()
	data := []byte("self-addressed memo")

	inst := BuildMemoInstruction(data, wallet)

	// Only the sender (signer) should be included.
	accounts := inst.Accounts()
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}

	if !accounts[0].PublicKey.Equals(wallet) {
		t.Errorf("expected account %s, got %s", wallet, accounts[0].PublicKey)
	}
}

func TestBuildMemoInstructionEmptyData(t *testing.T) {
	sender := solanago.NewWallet().PublicKey()
	data := []byte{}

	inst := BuildMemoInstruction(data, sender)

	instData, err := inst.Data()
	if err != nil {
		t.Fatalf("unexpected error getting instruction data: %v", err)
	}
	if len(instData) != 0 {
		t.Errorf("expected empty data, got %d bytes", len(instData))
	}
}

func TestBuildMemoInstructionMaxSize(t *testing.T) {
	sender := solanago.NewWallet().PublicKey()
	data := make([]byte, MaxMemoSize)
	for i := range data {
		data[i] = byte(i % 256)
	}

	inst := BuildMemoInstruction(data, sender)

	instData, err := inst.Data()
	if err != nil {
		t.Fatalf("unexpected error getting instruction data: %v", err)
	}
	if len(instData) != MaxMemoSize {
		t.Errorf("expected %d bytes, got %d", MaxMemoSize, len(instData))
	}
}

// --- MemoProgramID test ---

func TestMemoProgramID(t *testing.T) {
	expected := solanago.MustPublicKeyFromBase58("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")
	if !MemoProgramID.Equals(expected) {
		t.Errorf("MemoProgramID mismatch: got %s, want %s", MemoProgramID, expected)
	}
}

// --- ParseMemoTransaction tests ---

func TestParseMemoTransactionNilTx(t *testing.T) {
	memos := ParseMemoTransaction(nil, nil, "fakesig")
	if len(memos) != 0 {
		t.Errorf("expected 0 memos for nil transaction, got %d", len(memos))
	}
}

func TestParseMemoTransactionWithMemo(t *testing.T) {
	// Manually construct a transaction with a memo instruction.
	senderPubKey := solanago.NewWallet().PublicKey()

	memoData := []byte("test memo message")

	// Build the memo instruction.
	memoInst := BuildMemoInstruction(memoData, senderPubKey)

	// Create a transaction using a dummy blockhash.
	dummyHash := solanago.MustHashFromBase58("11111111111111111111111111111111")
	tx, err := solanago.NewTransaction(
		[]solanago.Instruction{memoInst},
		dummyHash,
		solanago.TransactionPayer(senderPubKey),
	)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	// Set up a block time for the transaction.
	blockTime := solanago.UnixTimeSeconds(1700000000)
	sigStr := "5wHu1qwD7q5iFgJ5gvjLHvLfxqQpoKBcB8t7eMbDzDnfaARnMfMEFcMJF7rdsCSmbbJMM1MNr1baFG3g5JhR9Znr"

	// Parse memo instructions.
	memos := ParseMemoTransaction(tx, &blockTime, sigStr)

	if len(memos) != 1 {
		t.Fatalf("expected 1 memo, got %d", len(memos))
	}

	memo := memos[0]

	// Verify sender.
	if !memo.Sender.Equals(senderPubKey) {
		t.Errorf("expected sender %s, got %s", senderPubKey, memo.Sender)
	}

	// Verify data.
	if string(memo.Data) != string(memoData) {
		t.Errorf("expected data %q, got %q", memoData, memo.Data)
	}

	// Verify timestamp.
	expectedTime := time.Unix(1700000000, 0)
	if !memo.Timestamp.Equal(expectedTime) {
		t.Errorf("expected timestamp %v, got %v", expectedTime, memo.Timestamp)
	}

	// Verify signature.
	if memo.Signature != sigStr {
		t.Errorf("expected signature %s, got %s", sigStr, memo.Signature)
	}
}

func TestParseMemoTransactionNoBlockTime(t *testing.T) {
	senderPubKey := solanago.NewWallet().PublicKey()

	memoData := []byte("memo without blocktime")
	memoInst := BuildMemoInstruction(memoData, senderPubKey)

	dummyHash := solanago.MustHashFromBase58("11111111111111111111111111111111")
	tx, err := solanago.NewTransaction(
		[]solanago.Instruction{memoInst},
		dummyHash,
		solanago.TransactionPayer(senderPubKey),
	)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	// Parse with nil blockTime.
	memos := ParseMemoTransaction(tx, nil, "testsig")

	if len(memos) != 1 {
		t.Fatalf("expected 1 memo, got %d", len(memos))
	}

	// Timestamp should be zero time when blockTime is nil.
	if !memos[0].Timestamp.IsZero() {
		t.Errorf("expected zero timestamp, got %v", memos[0].Timestamp)
	}
}

func TestParseMemoTransactionNonMemoInstruction(t *testing.T) {
	// Create a transaction with a non-memo instruction (System Program).
	senderPubKey := solanago.NewWallet().PublicKey()

	nonMemoInst := solanago.NewInstruction(
		solanago.SystemProgramID,
		solanago.AccountMetaSlice{
			solanago.Meta(senderPubKey).SIGNER().WRITE(),
		},
		[]byte{0, 0, 0, 0}, // dummy system program data
	)

	dummyHash := solanago.MustHashFromBase58("11111111111111111111111111111111")
	tx, err := solanago.NewTransaction(
		[]solanago.Instruction{nonMemoInst},
		dummyHash,
		solanago.TransactionPayer(senderPubKey),
	)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	memos := ParseMemoTransaction(tx, nil, "testsig")
	if len(memos) != 0 {
		t.Errorf("expected 0 memos for non-memo transaction, got %d", len(memos))
	}
}

// --- rpcToWsURL tests ---

func TestRpcToWsURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://api.devnet.solana.com", "wss://api.devnet.solana.com"},
		{"http://127.0.0.1:8899", "ws://127.0.0.1:8899"},
		{"http://localhost:8899", "ws://localhost:8899"},
		{"wss://already-ws.example.com", "wss://already-ws.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := rpcToWsURL(tt.input)
			if result != tt.expected {
				t.Errorf("rpcToWsURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// --- NewClient tests ---

func TestNewClientDefaults(t *testing.T) {
	client, err := NewClient("", "")
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if client.rpcURL != DefaultDevnetRPC {
		t.Errorf("expected default RPC URL %s, got %s", DefaultDevnetRPC, client.rpcURL)
	}

	expectedWs := "wss://api.devnet.solana.com"
	if client.wsURL != expectedWs {
		t.Errorf("expected WS URL %s, got %s", expectedWs, client.wsURL)
	}

	if client.proxyAddr != "" {
		t.Errorf("expected empty proxy address, got %s", client.proxyAddr)
	}

	if client.rpcClient == nil {
		t.Error("expected rpcClient to be initialized")
	}
}

func TestNewClientCustomRPC(t *testing.T) {
	customURL := "http://127.0.0.1:8899"
	client, err := NewClient(customURL, "")
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if client.rpcURL != customURL {
		t.Errorf("expected RPC URL %s, got %s", customURL, client.rpcURL)
	}

	expectedWs := "ws://127.0.0.1:8899"
	if client.wsURL != expectedWs {
		t.Errorf("expected WS URL %s, got %s", expectedWs, client.wsURL)
	}
}

func TestNewClientWithProxy(t *testing.T) {
	// Test that client creation with a proxy address succeeds structurally.
	// We use a non-routable address; the proxy is not actually connected.
	proxyAddr := "127.0.0.1:9050"
	client, err := NewClient("", proxyAddr)
	if err != nil {
		t.Fatalf("unexpected error creating client with proxy: %v", err)
	}

	if client.proxyAddr != proxyAddr {
		t.Errorf("expected proxy address %s, got %s", proxyAddr, client.proxyAddr)
	}
}

// --- MemoMessage struct tests ---

func TestMemoMessageFields(t *testing.T) {
	sender := solanago.NewWallet().PublicKey()
	data := []byte("test data")
	ts := time.Now()
	sig := "somesignature"

	msg := MemoMessage{
		Sender:    sender,
		Data:      data,
		Timestamp: ts,
		Signature: sig,
	}

	if !msg.Sender.Equals(sender) {
		t.Errorf("sender mismatch")
	}
	if string(msg.Data) != "test data" {
		t.Errorf("data mismatch")
	}
	if !msg.Timestamp.Equal(ts) {
		t.Errorf("timestamp mismatch")
	}
	if msg.Signature != sig {
		t.Errorf("signature mismatch")
	}
}

// --- Multiple memo instructions in a single transaction ---

func TestParseMemoTransactionMultipleMemos(t *testing.T) {
	senderPubKey := solanago.NewWallet().PublicKey()

	memo1Data := []byte("first memo")
	memo2Data := []byte("second memo")

	inst1 := BuildMemoInstruction(memo1Data, senderPubKey)
	inst2 := BuildMemoInstruction(memo2Data, senderPubKey)

	dummyHash := solanago.MustHashFromBase58("11111111111111111111111111111111")
	tx, err := solanago.NewTransaction(
		[]solanago.Instruction{inst1, inst2},
		dummyHash,
		solanago.TransactionPayer(senderPubKey),
	)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	blockTime := solanago.UnixTimeSeconds(1700000000)
	memos := ParseMemoTransaction(tx, &blockTime, "multisig")

	if len(memos) != 2 {
		t.Fatalf("expected 2 memos, got %d", len(memos))
	}

	if string(memos[0].Data) != string(memo1Data) {
		t.Errorf("first memo data mismatch: got %q, want %q", memos[0].Data, memo1Data)
	}
	if string(memos[1].Data) != string(memo2Data) {
		t.Errorf("second memo data mismatch: got %q, want %q", memos[1].Data, memo2Data)
	}
}

// --- Mixed instructions (memo + non-memo) ---

func TestParseMemoTransactionMixedInstructions(t *testing.T) {
	senderPubKey := solanago.NewWallet().PublicKey()

	memoData := []byte("the actual memo")
	memoInst := BuildMemoInstruction(memoData, senderPubKey)

	// A non-memo instruction.
	otherInst := solanago.NewInstruction(
		solanago.SystemProgramID,
		solanago.AccountMetaSlice{
			solanago.Meta(senderPubKey).SIGNER().WRITE(),
		},
		[]byte{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	)

	dummyHash := solanago.MustHashFromBase58("11111111111111111111111111111111")
	tx, err := solanago.NewTransaction(
		[]solanago.Instruction{otherInst, memoInst},
		dummyHash,
		solanago.TransactionPayer(senderPubKey),
	)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	memos := ParseMemoTransaction(tx, nil, "mixedsig")
	if len(memos) != 1 {
		t.Fatalf("expected 1 memo in mixed transaction, got %d", len(memos))
	}

	if string(memos[0].Data) != string(memoData) {
		t.Errorf("memo data mismatch: got %q, want %q", memos[0].Data, memoData)
	}
}
