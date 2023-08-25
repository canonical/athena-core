package common

import (
    "context"
    "sync"
    "time"
)

func RunOnInterval(ctx context.Context, lock *sync.Mutex, d time.Duration, f func(ctx *context.Context, interval time.Duration)) {
    ticker := time.Tick(d) // nolint:staticcheck
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker:
            lock.Lock()
            f(&ctx, d)
            lock.Unlock()
        }
    }
}
