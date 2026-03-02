package solana

import (
	"encoding/binary"

	solanago "github.com/gagliardetto/solana-go"
)

// MemoProgramID is the public key of the Solana Memo Program (v2).
var MemoProgramID = solanago.MustPublicKeyFromBase58("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")

// SystemProgramID is the Solana System Program.
var SystemProgramID = solanago.MustPublicKeyFromBase58("11111111111111111111111111111111")

// MaxMemoSize is the maximum size of memo data in bytes.
const MaxMemoSize = 512

// BuildMemoInstruction creates a Solana Memo instruction with the sender
// as the only account (signer). Memo Program v2 requires all listed
// accounts to be signers, so the recipient must NOT be included here.
func BuildMemoInstruction(data []byte, sender solanago.PublicKey) *solanago.GenericInstruction {
	accounts := solanago.AccountMetaSlice{
		solanago.Meta(sender).SIGNER().WRITE(),
	}

	return solanago.NewInstruction(
		MemoProgramID,
		accounts,
		data,
	)
}

// BuildTransferInstruction creates a System Program transfer instruction
// for 0 lamports. This makes the transaction appear in the recipient's
// transaction history without costing extra fees.
func BuildTransferInstruction(sender, recipient solanago.PublicKey) *solanago.GenericInstruction {
	// System Program Transfer instruction data: u32 instruction index (2) + u64 lamports (0)
	data := make([]byte, 12)
	binary.LittleEndian.PutUint32(data[0:4], 2) // Transfer instruction index
	binary.LittleEndian.PutUint64(data[4:12], 0) // 0 lamports

	accounts := solanago.AccountMetaSlice{
		solanago.Meta(sender).SIGNER().WRITE(),
		solanago.Meta(recipient).WRITE(),
	}

	return solanago.NewInstruction(
		SystemProgramID,
		accounts,
		data,
	)
}

// BuildSimpleMemoInstruction creates a Solana instruction targeting the
// Memo Program without any associated accounts. This is useful for
// constructing unsigned memo instructions or for testing.
func BuildSimpleMemoInstruction(data []byte) *solanago.GenericInstruction {
	return solanago.NewInstruction(
		MemoProgramID,
		solanago.AccountMetaSlice{},
		data,
	)
}
