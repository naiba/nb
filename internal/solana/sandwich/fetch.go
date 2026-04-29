// internal/solana/sandwich/fetch.go
package sandwich

import (
	"context"
	"fmt"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"golang.org/x/sync/errgroup"
)

// FetchContext pulls the block containing the user tx plus ±slotsRange neighboring blocks.
//
// Slots that fail to fetch are recorded in Context.SkippedSlots so a single
// dropped block does not fail the entire analysis.
func FetchContext(ctx context.Context, rpcURL string, userSig, userAddr, userMint string,
	slotsRange int) (*Context, error) {

	client := rpc.New(rpcURL)
	sig := solana.MustSignatureFromBase58(userSig)

	maxVer := uint64(0)
	txRes, err := client.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{
		MaxSupportedTransactionVersion: &maxVer,
	})
	if err != nil {
		return nil, fmt.Errorf("get user transaction: %w", err)
	}

	userSlot := txRes.Slot

	// Build the slot range [userSlot - N, userSlot + N], clamped at 0.
	var slots []uint64
	for i := -slotsRange; i <= slotsRange; i++ {
		s := int64(userSlot) + int64(i)
		if s < 0 {
			continue
		}
		slots = append(slots, uint64(s))
	}

	// Fetch blocks in parallel.
	var mu sync.Mutex
	blocks := make(map[uint64]*rpc.GetBlockResult, len(slots))
	var skipped []uint64

	g, gctx := errgroup.WithContext(ctx)
	// Throttle concurrency to avoid hammering public RPC endpoints.
	sem := make(chan struct{}, 3)
	for _, s := range slots {
		s := s
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-gctx.Done():
				return gctx.Err()
			}
			blk, err := client.GetBlockWithOpts(gctx, s, &rpc.GetBlockOpts{
				MaxSupportedTransactionVersion: &maxVer,
				Encoding:                       solana.EncodingBase64,
			})
			if err != nil {
				// Degrade gracefully: record as skipped instead of failing the whole analysis.
				mu.Lock()
				skipped = append(skipped, s)
				mu.Unlock()
				return nil
			}
			mu.Lock()
			blocks[s] = blk
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Derive the user's ATA. Token-2022 mints use a different program ID, so
	// detect the mint owner first and build the PDA accordingly.
	mintPk := solana.MustPublicKeyFromBase58(userMint)
	walletPk := solana.MustPublicKeyFromBase58(userAddr)
	mintInfo, err := client.GetAccountInfo(ctx, mintPk)
	if err != nil {
		return nil, fmt.Errorf("get mint account info: %w", err)
	}
	if mintInfo.Value == nil {
		return nil, fmt.Errorf("mint account not found")
	}
	var ata solana.PublicKey
	if mintInfo.Value.Owner.Equals(solana.Token2022ProgramID) {
		ata, _, err = solana.FindProgramAddress(
			[][]byte{walletPk[:], solana.Token2022ProgramID[:], mintPk[:]},
			solana.SPLAssociatedTokenAccountProgramID,
		)
	} else {
		ata, _, err = solana.FindAssociatedTokenAddress(walletPk, mintPk)
	}
	if err != nil {
		return nil, fmt.Errorf("derive user ATA: %w", err)
	}

	return &Context{
		UserSig:      userSig,
		UserSlot:     userSlot,
		UserAddr:     userAddr,
		UserMint:     userMint,
		UserTokenAcc: ata.String(),
		Blocks:       blocks,
		UserTx:       txRes,
		SkippedSlots: skipped,
	}, nil
}
