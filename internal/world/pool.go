package world

import (
	"context"
	"sync"
)

type job struct {
	x, z   int32
	result chan *Chunk
}

// Pool bounds both the number of generation goroutines and queued work.
type Pool struct {
	jobs   chan job
	done   chan struct{}
	mu     sync.RWMutex
	closed bool
}

func NewPool(workers, queue int) *Pool {
	if workers < 1 {
		workers = 1
	}
	if queue < 1 {
		queue = 1
	}
	p := &Pool{jobs: make(chan job, queue), done: make(chan struct{})}
	for i := 0; i < workers; i++ {
		go func() {
			for j := range p.jobs {
				j.result <- GenerateChunk(j.x, j.z)
				close(j.result)
			}
		}()
	}
	return p
}

func (p *Pool) Generate(ctx context.Context, x, z int32) (*Chunk, error) {
	result := make(chan *Chunk, 1)
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, context.Canceled
	}
	select {
	case p.jobs <- job{x: x, z: z, result: result}:
		p.mu.RUnlock()
	case <-ctx.Done():
		p.mu.RUnlock()
		return nil, ctx.Err()
	case <-p.done:
		p.mu.RUnlock()
		return nil, context.Canceled
	}
	select {
	case chunk := <-result:
		return chunk, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.done:
		return nil, context.Canceled
	}
}

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	close(p.done)
	close(p.jobs)
}
