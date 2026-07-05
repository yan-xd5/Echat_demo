package main

import (
	"sync"

	"github.com/panjf2000/ants/v2"
	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/message"
)

// sessionQueue 单个会话的消息队列。
type sessionQueue struct {
	msgs    []*message.Message
	running bool
}

// Pool ants 动态池 + 会话队列。
type Pool struct {
	pool     *ants.Pool
	mu       sync.Mutex
	sessions map[string]*sessionQueue
	pipeline *Pipeline
}

// NewPool 创建协程池。
func NewPool(pipeline *Pipeline) (*Pool, error) {
	pool, err := ants.NewPool(64,
		ants.WithExpiryDuration(30),
		ants.WithMaxBlockingTasks(65536),
		ants.WithPanicHandler(func(i interface{}) {
			log.Errorf("[Controller] worker panic: %v", i)
		}),
	)
	if err != nil {
		return nil, err
	}
	return &Pool{
		pool:     pool,
		sessions: make(map[string]*sessionQueue),
		pipeline: pipeline,
	}, nil
}

// Submit 将消息提交到对应 session 的队列。
func (p *Pool) Submit(sessionID string, msg *message.Message) error {
	p.mu.Lock()
	sq := p.sessions[sessionID]
	if sq == nil {
		sq = &sessionQueue{}
		p.sessions[sessionID] = sq
	}
	sq.msgs = append(sq.msgs, msg)

	if !sq.running {
		sq.running = true
		p.mu.Unlock()
		if err := p.pool.Submit(func() { p.processSession(sessionID) }); err != nil {
			p.mu.Lock()
			if s, ok := p.sessions[sessionID]; ok {
				s.running = false
				// 检查期间是否有新消息入队，有则重新提交
				if len(s.msgs) > 0 {
					s.running = true
					p.mu.Unlock()
					if err2 := p.pool.Submit(func() { p.processSession(sessionID) }); err2 != nil {
						p.mu.Lock()
						if s2, ok2 := p.sessions[sessionID]; ok2 {
							s2.running = false
						}
						p.mu.Unlock()
						log.Errorf("[Controller] 协程池重试提交失败: session=%s, err=%v", sessionID, err2)
						return err2
					}
					return nil
				}
			}
			p.mu.Unlock()
			return err
		}
		return nil
	}
	p.mu.Unlock()
	return nil
}

// processSession 循环消费同一 session 的消息。
func (p *Pool) processSession(sessionID string) {
	for {
		p.mu.Lock()
		sq := p.sessions[sessionID]
		if sq == nil || len(sq.msgs) == 0 {
			if sq != nil {
				sq.running = false
			}
			delete(p.sessions, sessionID)
			p.mu.Unlock()
			return
		}
		msg := sq.msgs[0]
		sq.msgs = sq.msgs[1:]
		p.mu.Unlock()

		p.pipeline.Process(msg)
	}
}

// Shutdown 关闭协程池。
func (p *Pool) Shutdown() { p.pool.Release() }
