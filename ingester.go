package geoengine

import (
	"context"
	"sync"
	"time"

	geopb "github.com/AlexG695/geo-engine-go/proto/geopb"
)

// AsyncIngester manages a lock-free channel queue for background batch ingestion.
type AsyncIngester struct {
	client    *Client
	queue     chan *geopb.LocationPing
	batchSize int
	interval  time.Duration
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewAsyncIngester instantiates a background worker pool with auto-flushing.
func NewAsyncIngester(client *Client, bufferSize, batchSize int, flushInterval time.Duration) *AsyncIngester {
	ctx, cancel := context.WithCancel(context.Background())
	ai := &AsyncIngester{
		client:    client,
		queue:     make(chan *geopb.LocationPing, bufferSize),
		batchSize: batchSize,
		interval:  flushInterval,
		ctx:       ctx,
		cancel:    cancel,
	}

	ai.wg.Add(1)
	go ai.worker()

	return ai
}

// Enqueue non-blockingly appends a location ping to the ingestion buffer.
func (ai *AsyncIngester) Enqueue(ping *geopb.LocationPing) bool {
	select {
	case ai.queue <- ping:
		return true
	default:
		return false // Queue buffer capacity full
	}
}

func (ai *AsyncIngester) worker() {
	defer ai.wg.Done()

	ticker := time.NewTicker(ai.interval)
	defer ticker.Stop()

	batch := make([]*geopb.LocationPing, 0, ai.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		toSend := make([]*geopb.LocationPing, len(batch))
		copy(toSend, batch)
		batch = batch[:0]

		go func(items []*geopb.LocationPing) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = ai.client.SendBatchLocationGRPC(ctx, items)
		}(toSend)
	}

	for {
		select {
		case <-ai.ctx.Done():
			for ping := range ai.queue {
				batch = append(batch, ping)
				if len(batch) >= ai.batchSize {
					flush()
				}
			}
			flush()
			return

		case ping, ok := <-ai.queue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, ping)
			if len(batch) >= ai.batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// Stop drains all remaining pending items and terminates workers.
func (ai *AsyncIngester) Stop() {
	ai.cancel()
	close(ai.queue)
	ai.wg.Wait()
}
