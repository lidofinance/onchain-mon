package feeder

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"testing"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/lidofinance/onchain-mon/internal/pkg/chain/entity"
)

// Swallows publishes so recovery can be exercised without a NATS server.
type noopJetStream struct{ jetstream.JetStream }

func (n *noopJetStream) PublishAsync(_ string, _ []byte, _ ...jetstream.PublishOpt) (jetstream.PubAckFuture, error) {
	return nil, nil
}

// The RPC answers fine but returns an empty block list: that used to leave
// latestPubBlock nil, and the caller dereferenced it right away.
type emptyChain struct{ ChainSrv }

func (e *emptyChain) GetLatestBlock(_ context.Context) (*entity.RpcResponse[entity.EthBlock], error) {
	b := entity.EthBlock{Number: "0x64"} // 100
	return &entity.RpcResponse[entity.EthBlock]{Result: &b}, nil
}
func (e *emptyChain) FetchBlocksInRange(_ context.Context, _, _ int64) (*entity.RpcResponse[[]entity.EthBlock], error) {
	empty := []entity.EthBlock{}
	return &entity.RpcResponse[[]entity.EthBlock]{Result: &empty}, nil
}

// A successful response carrying a nil Result.
type nilResultChain struct{ ChainSrv }

func (n *nilResultChain) GetLatestBlock(_ context.Context) (*entity.RpcResponse[entity.EthBlock], error) {
	return &entity.RpcResponse[entity.EthBlock]{Result: nil}, nil
}

func newTestFeeder(c ChainSrv) *Feeder {
	return &Feeder{
		log:      slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1})),
		chainSrv: c,
	}
}

func TestRecoverNoPanic(t *testing.T) {
	t.Run("empty_block_list", func(t *testing.T) {
		got, err := newTestFeeder(&emptyChain{}).recoverMissedBlocks(context.Background(), 50)
		if err == nil {
			t.Fatalf("expected an error, got block %v", got)
		}
		if got != nil {
			t.Fatalf("block must be nil on error, got %v", got)
		}
		t.Logf("got expected error: %v", err)
	})

	t.Run("nil_result", func(t *testing.T) {
		got, err := newTestFeeder(&nilResultChain{}).recoverMissedBlocks(context.Background(), 50)
		if err == nil {
			t.Fatalf("expected an error, got block %v", got)
		}
		t.Logf("got expected error: %v", err)
	})
}

// Records how the recovery splits a gap into chunks.
type chunkSpyChain struct {
	ChainSrv
	latest int64
	ranges [][2]int64
}

func (c *chunkSpyChain) GetLatestBlock(_ context.Context) (*entity.RpcResponse[entity.EthBlock], error) {
	b := entity.EthBlock{Number: "0x" + strconv.FormatInt(c.latest, 16)}
	return &entity.RpcResponse[entity.EthBlock]{Result: &b}, nil
}

func (c *chunkSpyChain) FetchBlocksInRange(_ context.Context, from, to int64) (*entity.RpcResponse[[]entity.EthBlock], error) {
	c.ranges = append(c.ranges, [2]int64{from, to})

	blocks := make([]entity.EthBlock, 0, to-from+1)
	for n := from; n <= to; n++ {
		blocks = append(blocks, entity.EthBlock{
			Number: "0x" + strconv.FormatInt(n, 16),
			Hash:   "0xhash" + strconv.FormatInt(n, 10),
		})
	}

	return &entity.RpcResponse[[]entity.EthBlock]{Result: &blocks}, nil
}

func (c *chunkSpyChain) FetchReceipts(_ context.Context, _ []string) (*entity.RpcResponse[[]entity.BlockReceipt], error) {
	empty := []entity.BlockReceipt{}
	return &entity.RpcResponse[[]entity.BlockReceipt]{Result: &empty}, nil
}

func Test_recover_splits_gap_into_chunks(t *testing.T) {
	// 120 missing blocks must become 50 + 50 + 20.
	spy := &chunkSpyChain{latest: 220}

	f := newTestFeeder(spy)
	f.js = &noopJetStream{}

	last, err := f.recoverMissedBlocks(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := [][2]int64{{101, 150}, {151, 200}, {201, 220}}
	if len(spy.ranges) != len(want) {
		t.Fatalf("expected %d chunks, got %d: %v", len(want), len(spy.ranges), spy.ranges)
	}
	for i := range want {
		if spy.ranges[i] != want[i] {
			t.Errorf("chunk %d: got %v, want %v", i, spy.ranges[i], want[i])
		}
	}
	if got := last.GetNumber(); got != 220 {
		t.Errorf("last recovered block: got %d, want 220", got)
	}
	t.Logf("chunks: %v", spy.ranges)
}

func Test_recover_single_chunk_when_gap_is_small(t *testing.T) {
	spy := &chunkSpyChain{latest: 105}

	f := newTestFeeder(spy)
	f.js = &noopJetStream{}

	if _, err := f.recoverMissedBlocks(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := [][2]int64{{101, 105}}
	if len(spy.ranges) != 1 || spy.ranges[0] != want[0] {
		t.Errorf("got %v, want %v", spy.ranges, want)
	}
}

func Test_recover_chunk_boundary_is_exact(t *testing.T) {
	// Exactly one chunk worth of blocks must not spill into a second request.
	spy := &chunkSpyChain{latest: 100 + RecoverChunkSize}

	f := newTestFeeder(spy)
	f.js = &noopJetStream{}

	if _, err := f.recoverMissedBlocks(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(spy.ranges) != 1 {
		t.Errorf("expected exactly 1 chunk for %d blocks, got %v", RecoverChunkSize, spy.ranges)
	}
}

// Fails the second chunk so the first one's progress must survive.
type failSecondChunkChain struct {
	chunkSpyChain
	calls int
}

func (c *failSecondChunkChain) FetchBlocksInRange(ctx context.Context, from, to int64) (*entity.RpcResponse[[]entity.EthBlock], error) {
	c.calls++
	if c.calls == 2 {
		return nil, errors.New("rpc is down")
	}

	return c.chunkSpyChain.FetchBlocksInRange(ctx, from, to)
}

func Test_recover_keeps_progress_when_a_later_chunk_fails(t *testing.T) {
	// 120 blocks => three chunks; the second one dies mid-recovery.
	spy := &failSecondChunkChain{chunkSpyChain: chunkSpyChain{latest: 220}}

	f := newTestFeeder(spy)
	f.js = &noopJetStream{}

	last, err := f.recoverMissedBlocks(context.Background(), 100)
	if err != nil {
		t.Fatalf("progress must be reported, not an error: %v", err)
	}
	if last == nil {
		t.Fatal("expected the last published block, got nil")
	}
	// The first chunk ended at 150 — resuming from there replays nothing.
	if got := last.GetNumber(); got != 150 {
		t.Errorf("resume point: got %d, want 150", got)
	}
}

// Fails every publish so nothing can be salvaged.
type failPublishFeeder struct{ jetstream.JetStream }

func (f *failPublishFeeder) PublishAsync(_ string, _ []byte, _ ...jetstream.PublishOpt) (jetstream.PubAckFuture, error) {
	return nil, errors.New("nats rejected the message")
}

func Test_recover_reports_error_when_nothing_was_published(t *testing.T) {
	spy := &chunkSpyChain{latest: 105}

	f := newTestFeeder(spy)
	f.js = &failPublishFeeder{}

	last, err := f.recoverMissedBlocks(context.Background(), 100)
	if err == nil {
		t.Fatalf("expected an error, got block %v", last)
	}
	if last != nil {
		t.Errorf("block must be nil when nothing was published, got %v", last)
	}
}
