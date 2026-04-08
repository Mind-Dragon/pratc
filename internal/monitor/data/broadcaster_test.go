package data

import (
	"context"
	"sync"
	"testing"
	"time"
)

type mockStore struct {
	activeJobs []SyncJobView
}

func (m *mockStore) GetActiveJobs() []SyncJobView {
	return m.activeJobs
}

type mockRateLimitFetcher struct {
	view RateLimitView
	err  error
}

func (m *mockRateLimitFetcher) Fetch(ctx context.Context) (RateLimitView, error) {
	return m.view, m.err
}

type mockTimelineAggregator struct {
	buckets []ActivityBucket
}

func (m *mockTimelineAggregator) GetTimeline(hours int) []ActivityBucket {
	return m.buckets
}

func TestBroadcasterSubscribe(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch := b.Subscribe()

	b.mu.Lock()
	_, ok := b.subscribers[ch]
	b.mu.Unlock()

	if !ok {
		t.Error("Subscribe() should register channel in subscribers map")
	}
}

func TestBroadcasterUnsubscribe(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch := b.Subscribe()

	b.Unsubscribe(ch)

	b.mu.Lock()
	_, ok := b.subscribers[ch]
	b.mu.Unlock()

	if ok {
		t.Error("Unsubscribe() should remove channel from subscribers map")
	}
}

func TestBroadcasterUnsubscribeNotSubscribed(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch := make(chan DataUpdate)

	b.Unsubscribe(ch)
}

func TestBroadcasterMultipleSubscribers(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)

	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	ch3 := b.Subscribe()

	b.mu.Lock()
	count := len(b.subscribers)
	b.mu.Unlock()

	if count != 3 {
		t.Errorf("expected 3 subscribers, got %d", count)
	}

	b.Unsubscribe(ch1)
	b.Unsubscribe(ch2)
	b.Unsubscribe(ch3)

	b.mu.Lock()
	count = len(b.subscribers)
	b.mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 subscribers after all unsubscribes, got %d", count)
	}
}

func TestBroadcasterStartStop(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ctx := context.Background()

	b.Start(ctx)

	b.mu.Lock()
	running := b.running
	b.mu.Unlock()

	if !running {
		t.Error("Start() should set running to true")
	}

	b.Stop()

	b.mu.Lock()
	running = b.running
	b.mu.Unlock()

	if running {
		t.Error("Stop() should set running to false")
	}
}

func TestBroadcasterStartTwice(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ctx := context.Background()

	b.Start(ctx)

	b.mu.Lock()
	running1 := b.running
	b.mu.Unlock()

	b.Start(ctx)

	b.mu.Lock()
	running2 := b.running
	b.mu.Unlock()

	if !running1 || !running2 {
		t.Error("broadcaster should be running after Start()")
	}

	b.Stop()
}

func TestBroadcasterBroadcast(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch := b.Subscribe()
	_ = ch // channel registered via subscribe

	update := DataUpdate{
		Timestamp: time.Now(),
		SyncJobs:  []SyncJobView{{ID: "job1", Repo: "owner/repo", Status: StatusActive}},
	}

	b.broadcast(update)

	select {
	case received := <-ch:
		if received.SyncJobs[0].ID != "job1" {
			t.Errorf("expected job1, got %s", received.SyncJobs[0].ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("broadcast should have sent update to channel")
	}
}

func TestBroadcasterMultipleSubscribersReceiveSameData(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	ch3 := b.Subscribe()

	update := DataUpdate{
		Timestamp: time.Now(),
		SyncJobs:  []SyncJobView{{ID: "job1", Repo: "owner/repo", Status: StatusActive}},
	}

	b.broadcast(update)

	for i, ch := range []chan DataUpdate{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			if received.SyncJobs[0].ID != "job1" {
				t.Errorf("subscriber %d: expected job1, got %s", i, received.SyncJobs[0].ID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d should have received update", i)
		}
	}
}

func TestBroadcasterStopClosesAllChannels(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	ctx := context.Background()
	b.Start(ctx)
	b.Stop()

	select {
	case _, ok := <-ch1:
		if ok {
			t.Error("channel 1 should be closed after Stop()")
		}
	default:
		t.Error("channel 1 should be closed and not hit default")
	}

	select {
	case _, ok := <-ch2:
		if ok {
			t.Error("channel 2 should be closed after Stop()")
		}
	default:
		t.Error("channel 2 should be closed and not hit default")
	}
}

func TestBroadcasterUnsubscribeRemovesChannel(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	b.Unsubscribe(ch1)

	b.mu.Lock()
	_, ch1StillSubscribed := b.subscribers[ch1]
	_, ch2StillSubscribed := b.subscribers[ch2]
	b.mu.Unlock()

	if ch1StillSubscribed {
		t.Error("ch1 should not be in subscribers after Unsubscribe")
	}

	if !ch2StillSubscribed {
		t.Error("ch2 should still be in subscribers")
	}

	update := DataUpdate{
		Timestamp: time.Now(),
		SyncJobs:  []SyncJobView{{ID: "job1"}},
	}

	b.broadcast(update)

	select {
	case <-ch1:
		t.Error("unsubscribed channel should not receive update")
	default:
	}

	select {
	case received := <-ch2:
		if received.SyncJobs[0].ID != "job1" {
			t.Errorf("expected job1, got %s", received.SyncJobs[0].ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("active channel should receive update")
	}
}

func TestBroadcasterConcurrentAccess(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ctx := context.Background()
	b.Start(ctx)

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := b.Subscribe()
			time.Sleep(10 * time.Millisecond)
			b.Unsubscribe(ch)
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			update := DataUpdate{Timestamp: time.Now()}
			b.broadcast(update)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()

	b.Stop()
}
