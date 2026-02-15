# Gitea Sync Controller — Line-by-Line Go Explanation

Reference file: `gitea-service/internal/sync/controller.go`

---

## Line 1 — `package sync`

Every Go file starts with a **package declaration**. This file belongs to the `sync` package. All files in the same folder must share the same package name. Other code imports this as `"github.com/devplatform/gitea-service/internal/sync"`.

---

## Lines 3-25 — `import (...)`

Go groups imports in a block. There are two groups separated by a blank line:

**Standard library (lines 4-17):**
- `"context"` — For cancellation/timeouts. Go passes a `context.Context` through function chains so you can cancel long operations.
- `"crypto/hmac"` + `"crypto/sha256"` — Used to verify webhook signatures (HMAC-SHA256). This is the "no trust" policy: prove the request came from Gitea.
- `"encoding/hex"` — Converts binary bytes to hex strings (like `a3f2b1...`).
- `"encoding/json"` — Serialize/deserialize structs to/from JSON.
- `"fmt"` — String formatting (`Sprintf`, `Errorf`).
- `"io"` — I/O primitives. `io.ReadAll` reads an entire HTTP body.
- `"net/http"` — Go's built-in HTTP server and client.
- `"net/url"` — URL encoding for form data (Keycloak token request).
- `"os"` — File system operations (`ReadFile`, `WriteFile`, `Rename`, `MkdirAll`).
- `"path/filepath"` — Cross-platform file path joining.
- `"strings"` — String utilities (`strings.NewReader` to turn a string into an `io.Reader`).
- `"sync"` — Concurrency primitives (`sync.Mutex`, `sync.WaitGroup`). Note: this is the Go standard `sync` package, different from our `package sync`.
- `"time"` — Durations, timers, tickers.

**Third-party (lines 19-24):**
- `config` / `gitea` — Our own internal packages.
- `prometheus` / `promauto` / `promhttp` — Prometheus client library for exposing metrics.
- `logrus` — Structured logging library (outputs JSON logs with fields).

---

## Lines 27-59 — Prometheus Metrics (package-level `var` block)

```go
var (
    syncTotal = promauto.NewCounterVec(...)
    ...
)
```

`var (...)` declares **package-level variables** — created once when the program starts.

**Line 29-35 — `syncTotal`**: A **CounterVec** (a counter that goes up, never down, with labels). Labels are `"type"` and `"status"`. Used like `syncTotal.WithLabelValues("webhook", "success").Inc()`. This lets Prometheus track: "how many webhook syncs succeeded vs failed vs how many reconcile syncs succeeded vs failed."

**Line 37-44 — `syncDuration`**: A **HistogramVec** — records how long operations take. `Buckets: prometheus.DefBuckets` gives default bucket sizes (0.005s, 0.01s, 0.025s, ... 10s). Prometheus can then answer "what percentage of syncs took under 1 second?"

**Line 46-51 — `syncLastSuccess`**: A **Gauge** — a single number that goes up or down. Stores the Unix timestamp of the last successful full reconciliation. Useful for alerting: "if this timestamp is older than 15 minutes, something is wrong."

**Line 53-58 — `retryQueueSize`**: Another Gauge tracking how many items are waiting for retry.

`promauto` auto-registers these with Prometheus's global registry, so they appear at `/metrics` automatically.

---

## Lines 61-64 — Constants

```go
const (
    maxRetries    = 5
    retryQueueCap = 100
)
```

`const` declares compile-time constants. `maxRetries = 5` means we try syncing a user 5 times before giving up. `retryQueueCap = 100` limits the queue to 100 items (backpressure protection).

---

## Lines 66-71 — `retryItem` struct

```go
type retryItem struct {
    UID       string    `json:"uid"`
    Attempts  int       `json:"attempts"`
    NextRetry time.Time `json:"next_retry"`
}
```

A **struct** is like a class with fields but no methods (methods are defined separately in Go).

- `UID string` — the Gitea username whose sync failed.
- `Attempts int` — how many times we've retried.
- `NextRetry time.Time` — when to try next (exponential backoff).

The backtick parts `` `json:"uid"` `` are **struct tags**. They tell `encoding/json` what field name to use when converting to/from JSON. Without them, JSON would use `"UID"` (uppercase). We need these because the state file is persisted to disk.

---

## Lines 73-77 — `persistedState` struct

```go
type persistedState struct {
    RetryItems           []retryItem `json:"retry_items"`
    LastReconcileSuccess time.Time   `json:"last_reconcile_success"`
}
```

This is the shape of what gets written to `/data/state.json`. When the pod restarts, this file is loaded back to restore:
- The retry queue (items that were waiting to be retried)
- When the last successful full reconciliation happened

---

## Lines 79-92 — `backoffDuration` function

```go
func backoffDuration(attempt int) time.Duration {
    durations := []time.Duration{
        5 * time.Second,
        15 * time.Second,
        45 * time.Second,
        2 * time.Minute,
        5 * time.Minute,
    }
    if attempt >= len(durations) {
        return durations[len(durations)-1]
    }
    return durations[attempt]
}
```

A **standalone function** (not attached to a struct). Takes an attempt number, returns how long to wait.

- `[]time.Duration{...}` — a **slice literal** (like an array but dynamic size).
- `5 * time.Second` — Go's `time.Duration` type. You multiply a number by a time unit.
- `len(durations)` — built-in function to get slice length.
- If attempt is 0 -> 5s, 1 -> 15s, 2 -> 45s, 3 -> 2min, 4 -> 5min, 5+ -> 5min (capped).

This is **exponential backoff** — each retry waits longer, so we don't hammer a failing service.

---

## Lines 94-108 — `Controller` struct

```go
type Controller struct {
    giteaService *gitea.Service    // business logic for syncing repos
    giteaClient  *gitea.Client     // low-level Gitea API client
    cfg          *config.Config    // all env var configuration
    logger       *logrus.Logger    // structured logger

    retryMu              sync.Mutex   // protects retryItems and lastReconcileSuccess
    retryItems           []retryItem  // the retry queue
    lastReconcileSuccess time.Time    // when last full reconcile succeeded
    dataDir              string       // where to persist state (/data)

    stopCh chan struct{}     // signal channel to stop goroutines
    wg     sync.WaitGroup   // wait for goroutines to finish
}
```

This is the **core type** of the file. All the methods below are attached to it.

Key Go concepts here:

- `*gitea.Service` — a **pointer** to a `Service` struct from the `gitea` package. The `*` means "this is a reference, not a copy." Almost everything in Go is passed by pointer for efficiency.
- `sync.Mutex` — a **mutual exclusion lock**. When multiple goroutines (threads) access `retryItems`, the mutex ensures only one at a time can read/write. You call `.Lock()` before and `.Unlock()` after.
- `chan struct{}` — a **channel** of empty structs. Channels are Go's way for goroutines to communicate. An empty struct `struct{}` takes zero memory — it's used purely as a signal. When you `close(stopCh)`, every goroutine listening on it wakes up and knows to exit.
- `sync.WaitGroup` — a counter that tracks running goroutines. You call `wg.Add(1)` before launching a goroutine and `wg.Done()` when it finishes. `wg.Wait()` blocks until the counter reaches zero.

---

## Lines 110-126 — `NewController` (constructor)

```go
func NewController(
    giteaService *gitea.Service,
    giteaClient *gitea.Client,
    cfg *config.Config,
    logger *logrus.Logger,
) *Controller {
    return &Controller{
        giteaService: giteaService,
        ...
        retryItems:   make([]retryItem, 0),
        dataDir:      cfg.DataDir,
        stopCh:       make(chan struct{}),
    }
}
```

Go doesn't have constructors. By convention, you create a `New___()` function that returns a pointer.

- `&Controller{...}` — creates a `Controller` on the heap and returns a pointer to it. `&` means "take the address of."
- `make([]retryItem, 0)` — `make` allocates and initializes Go built-in types (slices, maps, channels). This creates an empty slice with length 0.
- `make(chan struct{})` — creates an **unbuffered channel**. It blocks on send until someone receives (but we only use `close()` on it, which is non-blocking).

---

## Lines 128-131 — `stateFilePath`

```go
func (c *Controller) stateFilePath() string {
    return filepath.Join(c.dataDir, "state.json")
}
```

`func (c *Controller)` — this is a **method receiver**. It means this function is attached to the `Controller` type. `c` is like `this` or `self` in other languages. `filepath.Join` joins path segments with the OS-appropriate separator (`/` on Linux).

---

## Lines 133-170 — `loadState`

```go
func (c *Controller) loadState() {
    path := c.stateFilePath()

    data, err := os.ReadFile(path)
```

Go functions return **multiple values**. `os.ReadFile` returns `([]byte, error)`. You always check the `error`:

```go
    if err != nil {
        if os.IsNotExist(err) {
            // First boot — no state file yet, that's normal
            return
        }
        // Some other error (permissions, disk failure)
        c.logger.WithError(err).Warn("Failed to read persisted state")
        return
    }
```

`os.IsNotExist(err)` checks if the error is specifically "file not found." This is expected on first boot.

```go
    var state persistedState
    if err := json.Unmarshal(data, &state); err != nil {
```

`json.Unmarshal(data, &state)` — parses JSON bytes into the struct. The `&state` passes a pointer so `Unmarshal` can fill in the fields. The `:=` inside the `if` is Go's **short variable declaration** — it declares `err` scoped to this `if` block (separate from the `err` above).

```go
    c.retryMu.Lock()
    c.retryItems = state.RetryItems
    if c.retryItems == nil {
        c.retryItems = make([]retryItem, 0)
    }
```

Lock the mutex before touching shared data. If `RetryItems` was `null` in JSON, it deserializes as `nil` in Go. We normalize it to an empty slice so the rest of the code doesn't need nil checks.

```go
    retryQueueSize.Set(float64(len(c.retryItems)))
    c.retryMu.Unlock()
```

Update the Prometheus gauge, then release the lock. Prometheus gauges take `float64`, so we convert with `float64(...)`.

```go
    if !state.LastReconcileSuccess.IsZero() {
        syncLastSuccess.Set(float64(state.LastReconcileSuccess.Unix()))
    }
```

`time.Time` zero value is `0001-01-01 00:00:00`. `.IsZero()` checks if it was never set. `.Unix()` converts to seconds since 1970.

```go
    c.logger.WithFields(logrus.Fields{
        "retry_items":    len(state.RetryItems),
        "last_reconcile": state.LastReconcileSuccess.Format(time.RFC3339),
    }).Info("Restored persisted state")
```

**Structured logging** with logrus. `WithFields` adds key-value pairs to the log entry. The output will be JSON like:
```json
{"retry_items": 3, "last_reconcile": "2026-02-10T12:00:00Z", "message": "Restored persisted state", "level": "info"}
```

---

## Lines 172-209 — `saveState`

```go
func (c *Controller) saveState() {
    c.retryMu.Lock()
    state := persistedState{
        RetryItems:           make([]retryItem, len(c.retryItems)),
        LastReconcileSuccess: c.lastReconcileSuccess,
    }
    copy(state.RetryItems, c.retryItems)
    c.retryMu.Unlock()
```

We hold the lock **only long enough to copy the data**, then release it. `copy(dst, src)` is a built-in that copies slice elements. This avoids holding the lock during the slow file I/O below.

```go
    data, err := json.MarshalIndent(state, "", "  ")
```

`MarshalIndent` converts the struct to pretty-printed JSON bytes. Args: the value, a prefix (empty), and an indent string (2 spaces).

```go
    if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
```

`os.MkdirAll` is like `mkdir -p` — creates the directory and all parents. `0750` is the Unix permission: owner=rwx, group=r-x, others=---. `filepath.Dir` gets the parent directory from a path.

```go
    // Write atomically: temp file then rename
    tmpPath := path + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0640); err != nil {
```

**Atomic write pattern**: Write to a `.tmp` file first, then rename. If the process crashes mid-write, you lose the `.tmp` but the original `state.json` is still intact. `0640` = owner=rw, group=r, others=---. `os.WriteFile` creates the file, writes all data, and closes it in one call.

```go
    if err := os.Rename(tmpPath, path); err != nil {
```

`os.Rename` is an atomic operation on Linux (same filesystem). The file appears complete or not at all.

---

## Lines 211-239 — `Start`

```go
func (c *Controller) Start() {
    if !c.cfg.ReconcileEnabled {
        c.logger.Info("Reconciliation controller is disabled")
        return
    }
```

Early return pattern — if reconciliation is disabled via config, do nothing.

```go
    c.loadState()
```

Restore persisted state before launching goroutines.

```go
    c.wg.Add(1)
    go c.reconcileLoop()
```

`c.wg.Add(1)` — tells the WaitGroup "one more goroutine is starting."
`go c.reconcileLoop()` — the `go` keyword launches a **goroutine** (lightweight thread). This runs `reconcileLoop` concurrently. It's non-blocking — execution continues immediately.

Three goroutines are launched:
1. `reconcileLoop` — full sync every 5 minutes
2. `webhookHealthLoop` — checks webhook exists every 2 minutes
3. `retryLoop` — processes retry queue every 5 seconds

---

## Lines 241-248 — `Stop`

```go
func (c *Controller) Stop() {
    close(c.stopCh)
    c.wg.Wait()
    c.saveState()
}
```

`close(c.stopCh)` — **closes the channel**. In Go, when you close a channel, every goroutine doing `<-c.stopCh` in a `select` immediately receives the zero value. This is the standard "broadcast shutdown" pattern.

`c.wg.Wait()` — blocks until all 3 goroutines have called `wg.Done()` (meaning they've fully exited).

`c.saveState()` — persist the final state to disk before the process exits.

---

## Lines 250-278 — `EnqueueRetry`

```go
func (c *Controller) EnqueueRetry(uid string) {
    c.retryMu.Lock()

    for _, item := range c.retryItems {
        if item.UID == uid {
            c.retryMu.Unlock()
            return
        }
    }
```

`for _, item := range c.retryItems` — Go's **range loop**. It iterates over the slice. `_` discards the index (we only want the value). If the UID is already in the queue, we unlock and return early (deduplicate).

```go
    if len(c.retryItems) >= retryQueueCap {
        c.retryItems = c.retryItems[1:]
    }
```

If queue is full, drop the oldest item. `c.retryItems[1:]` is a **slice expression** — creates a new slice starting from index 1 (skipping index 0). This is like Python's `list[1:]`.

```go
    c.retryItems = append(c.retryItems, retryItem{
        UID:       uid,
        Attempts:  0,
        NextRetry: time.Now().Add(backoffDuration(0)),
    })
```

`append` is a built-in that adds to a slice. `retryItem{...}` is a **struct literal** — creates a new instance with named fields. `time.Now().Add(5 * time.Second)` means "5 seconds from now."

```go
    retryQueueSize.Set(float64(len(c.retryItems)))
    c.retryMu.Unlock()

    c.logger.WithField("uid", uid).Info("Enqueued user for retry sync")
    c.saveState()
```

Notice: we unlock **before** calling `saveState()` because `saveState` also acquires the lock internally. If we didn't unlock first, it would **deadlock** (the goroutine waits for a lock it already holds). This is why we didn't use `defer c.retryMu.Unlock()` here — we need precise control over when the unlock happens.

---

## Lines 280-293 — `SetupHTTPHandlers`

```go
func (c *Controller) SetupHTTPHandlers(mux *http.ServeMux) {
    mux.HandleFunc("/webhook/gitea", c.webhookHandler)
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    })
    mux.Handle("/metrics", promhttp.Handler())
}
```

`http.ServeMux` is Go's built-in HTTP router (like Express in Node.js).

- `HandleFunc` registers a handler function for a URL pattern.
- `c.webhookHandler` — passes the method as a function value (Go methods are first-class).
- `func(w, r) { ... }` — an **anonymous function** (lambda/closure) for the health endpoint.
- `json.NewEncoder(w).Encode(...)` — creates a JSON encoder that writes directly to the HTTP response writer. More efficient than `json.Marshal` + `w.Write` because it doesn't allocate an intermediate buffer.
- `map[string]string{"status": "ok"}` — a **map literal**. Maps are Go's hash maps / dictionaries.
- `promhttp.Handler()` — returns an `http.Handler` (interface) that serves Prometheus metrics. Note: `Handle` not `HandleFunc` because it takes a Handler interface, not a func.

---

## Lines 295-406 — `webhookHandler`

```go
func (c *Controller) webhookHandler(w http.ResponseWriter, r *http.Request) {
```

This is the HTTP handler signature Go expects: `(ResponseWriter, *Request)`.

```go
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
```

`http.Error` is a helper that writes an error response with the given status code. `http.MethodPost` is the constant `"POST"`. `http.StatusMethodNotAllowed` is `405`.

```go
    body, err := io.ReadAll(r.Body)
```

Read the entire request body into a byte slice. We need the raw bytes for HMAC verification AND for JSON parsing, so we read it once and reuse.

```go
    // Always validate webhook signature - no trust policy
    if c.cfg.GiteaWebhookSecret == "" {
        http.Error(w, "webhook secret not configured", http.StatusInternalServerError)
        return
    }
```

**No trust policy**: if the webhook secret isn't configured, we refuse ALL webhooks. We never accept unsigned requests.

```go
    sig := r.Header.Get("X-Gitea-Signature")
    if !verifyWebhookSignature(body, sig, c.cfg.GiteaWebhookSecret) {
```

Gitea sends the HMAC-SHA256 signature in the `X-Gitea-Signature` header. We verify it matches. If it doesn't, someone is sending fake requests.

```go
    eventType := r.Header.Get("X-Gitea-Event")
    if eventType != "repository" {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "ignored", "event": eventType})
        return
    }
```

Gitea sends many event types (push, issue, etc.). We only care about `"repository"` events (repo created/deleted). We return 200 OK for others so Gitea doesn't think the webhook is broken.

```go
    var payload struct {
        Action string `json:"action"`
        Sender struct {
            Login string `json:"login"`
        } `json:"sender"`
        Repository struct {
            Owner struct {
                Login string `json:"login"`
            } `json:"owner"`
            FullName string `json:"full_name"`
        } `json:"repository"`
    }
```

**Anonymous struct** — you can define a struct type inline. This is useful when you only need the type once and only care about a few fields from a large JSON payload. `json.Unmarshal` ignores any JSON fields that don't match struct fields.

```go
    serviceToken, err := c.getKeycloakToken()
```

Get an OAuth2 token from Keycloak so we can authenticate with the LDAP Manager service. This is a **client credentials grant** — machine-to-machine auth.

```go
    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
    defer cancel()
```

**Context with timeout**: creates a new context that automatically cancels after 30 seconds. `r.Context()` is the parent context from the HTTP request. `defer cancel()` ensures the context's resources are freed when this function returns. `defer` runs the function call when the enclosing function returns, regardless of how it returns (normal return, early return, etc.).

```go
    result, err := c.giteaService.SyncGiteaReposToLDAP(ctx, ownerLogin, serviceToken)
    duration := time.Since(start).Seconds()
    syncDuration.WithLabelValues("webhook").Observe(duration)
```

Call the actual sync logic. Measure how long it took. Record it in the Prometheus histogram.

```go
    if err != nil {
        ...
        c.EnqueueRetry(ownerLogin)
        http.Error(w, "sync failed", http.StatusInternalServerError)
        return
    }
```

If sync fails, don't lose the event — put the user in the retry queue. The retry loop will try again with exponential backoff.

```go
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":     "synced",
        "uid":        result.UID,
        "reposCount": result.ReposCount,
    })
```

`map[string]interface{}` — a map with string keys and **any type** values. `interface{}` is Go's "any" type (in Go 1.18+ you can also write `any`). Needed here because the values are mixed types (strings and int).

---

## Lines 408-432 — `reconcileLoop`

```go
func (c *Controller) reconcileLoop() {
    defer c.wg.Done()
```

`defer c.wg.Done()` — when this goroutine exits (for any reason), decrement the WaitGroup counter. This is critical for graceful shutdown.

```go
    select {
    case <-time.After(30 * time.Second):
    case <-c.stopCh:
        return
    }
```

**`select` statement** — Go's way to wait on multiple channels simultaneously. It blocks until one of the cases is ready:
- `time.After(30s)` returns a channel that receives a value after 30 seconds. This is a "wait 30 seconds" that can be interrupted.
- `<-c.stopCh` — if the stop channel is closed during the wait, exit immediately.

This is a **cancellable sleep**. Without `select`, if you did `time.Sleep(30s)`, you couldn't stop the goroutine during those 30 seconds.

```go
    ticker := time.NewTicker(c.cfg.ReconcileInterval)
    defer ticker.Stop()
```

`NewTicker` creates a channel that delivers a value periodically (every 5 minutes). `defer ticker.Stop()` releases the ticker's resources when the function exits.

```go
    for {
        select {
        case <-ticker.C:
            c.runFullReconcile()
        case <-c.stopCh:
            return
        }
    }
```

**Infinite loop with select** — this is the standard Go pattern for a periodic goroutine:
- `ticker.C` fires every 5 minutes -> run reconciliation
- `stopCh` closes -> exit the loop and goroutine

---

## Lines 434-473 — `runFullReconcile`

```go
    token, err := c.getKeycloakToken()
    if err != nil {
        syncTotal.WithLabelValues("reconcile", "error").Inc()
        return
    }
```

Get auth token first. If Keycloak is down, we can't sync — increment the error counter and try again next cycle.

```go
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()
```

`context.Background()` — the root context (not tied to any HTTP request). We give it a 2-minute timeout because full reconciliation touches all users and can take a while.

```go
    results, err := c.giteaService.SyncAllGiteaReposToLDAP(ctx, token)
```

Syncs ALL Gitea repos to ALL LDAP users. Returns a slice of results.

```go
    now := time.Now()
    syncLastSuccess.Set(float64(now.Unix()))

    c.retryMu.Lock()
    c.lastReconcileSuccess = now
    c.retryMu.Unlock()
```

Record success: update the Prometheus gauge AND the in-memory timestamp (protected by mutex). Then save to disk:

```go
    c.saveState()
```

---

## Lines 475-510 — `webhookHealthLoop` + `ensureWebhook`

Same pattern as `reconcileLoop`: initial delay (10s) -> run once -> then run on a ticker (every 2 minutes).

```go
func (c *Controller) ensureWebhook() {
    targetURL := fmt.Sprintf("http://%s/webhook/gitea", c.cfg.WebhookTargetHost)
    if err := c.giteaClient.EnsureWebhook(targetURL, c.cfg.GiteaWebhookSecret); err != nil {
```

`fmt.Sprintf` — string formatting. `%s` is replaced by the value. This builds the full webhook URL like `http://gitea-sync-controller.dev-platform.svc.cluster.local:8081/webhook/gitea`.

`EnsureWebhook` is **idempotent**: it lists existing webhooks, checks if ours exists, creates it only if missing. Safe to call repeatedly.

---

## Lines 512-527 — `retryLoop`

Same pattern but with a 5-second ticker. Every 5 seconds, check if any retry items are ready.

---

## Lines 529-602 — `processRetryQueue`

```go
    c.retryMu.Lock()
    now := time.Now()

    ready := make([]retryItem, 0)
    remaining := make([]retryItem, 0)

    for _, item := range c.retryItems {
        if now.After(item.NextRetry) {
            ready = append(ready, item)
        } else {
            remaining = append(remaining, item)
        }
    }

    c.retryItems = remaining
    c.retryMu.Unlock()
```

**Split the queue into two lists** under the lock:
- `ready` — items whose `NextRetry` time has passed (process now)
- `remaining` — items still waiting (keep in queue)

We replace `retryItems` with only the remaining ones, then unlock. The actual HTTP calls happen outside the lock (never hold a lock during network I/O).

```go
    if len(ready) == 0 {
        return
    }
```

Nothing to do, exit early.

```go
    token, err := c.getKeycloakToken()
    if err != nil {
        c.retryMu.Lock()
        c.retryItems = append(c.retryItems, ready...)
        c.retryMu.Unlock()
        return
    }
```

If we can't get a token, put all items back in the queue. `ready...` is the **spread operator** — expands a slice into individual arguments for `append`.

```go
    for _, item := range ready {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

        _, err := c.giteaService.SyncGiteaReposToLDAP(ctx, item.UID, token)
        cancel()
```

Note: `cancel()` is called immediately after the operation, not deferred. In a loop, `defer` would pile up until the function exits — calling `cancel()` explicitly is better.

```go
        if err != nil {
            item.Attempts++

            if item.Attempts >= maxRetries {
                // Give up after 5 attempts
                continue
            }

            item.NextRetry = time.Now().Add(backoffDuration(item.Attempts))
            c.retryMu.Lock()
            c.retryItems = append(c.retryItems, item)
            c.retryMu.Unlock()
```

If sync fails: increment attempts. If we've hit the max (5), `continue` skips to the next item (effectively dropping this one). Otherwise, calculate the next retry time with exponential backoff and re-add to the queue.

```go
    retryQueueSize.Set(float64(len(c.retryItems)))
    c.saveState()
```

After processing all ready items, update the metric and persist to disk.

---

## Lines 604-652 — `getKeycloakToken`

```go
func (c *Controller) getKeycloakToken() (string, error) {
```

Returns `(string, error)` — the Go convention for fallible operations. Callers always check `if err != nil`.

```go
    data := url.Values{}
    data.Set("grant_type", "client_credentials")
    data.Set("client_id", c.cfg.KeycloakClientID)
    data.Set("client_secret", c.cfg.KeycloakClientSecret)
```

`url.Values` is a `map[string][]string` with helper methods. It builds URL-encoded form data (`grant_type=client_credentials&client_id=...`). This is the **OAuth2 client credentials grant** — the controller authenticates itself (not a user) to Keycloak.

```go
    resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
```

`http.Post` sends a POST request. `data.Encode()` converts to `key=value&key=value` format. `strings.NewReader` wraps the string as an `io.Reader` (which `http.Post` expects for the body).

```go
    defer resp.Body.Close()
```

**Always close HTTP response bodies**. `defer` ensures it happens even if we return early due to an error.

```go
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return "", fmt.Errorf("Keycloak token request failed: status %d, body: %s", resp.StatusCode, string(body))
    }
```

`fmt.Errorf` creates an error value with a formatted message. `string(body)` converts `[]byte` to `string`.

```go
    var tokenResp struct {
        AccessToken string `json:"access_token"`
        ExpiresIn   int    `json:"expires_in"`
        TokenType   string `json:"token_type"`
    }
```

Another anonymous struct — we only need 3 fields from Keycloak's JSON response.

```go
    return tokenResp.AccessToken, nil
```

`nil` for the error means "no error" — success.

---

## Lines 654-665 — `verifyWebhookSignature`

```go
func verifyWebhookSignature(payload []byte, signature string, secret string) bool {
```

This is a **standalone function** (no receiver) — it doesn't need Controller state, it's a pure function.

```go
    if signature == "" {
        return false
    }
```

No signature at all = definitely invalid.

```go
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    expectedMAC := hex.EncodeToString(mac.Sum(nil))
```

**HMAC-SHA256 verification**:
1. `hmac.New(sha256.New, key)` — creates an HMAC hasher using SHA-256 with the shared secret.
2. `mac.Write(payload)` — feed the webhook body into the hasher.
3. `mac.Sum(nil)` — finalize and get the raw bytes. `nil` means "don't append to anything, just return the hash."
4. `hex.EncodeToString` — convert raw bytes to hex string (like `a3f2b1c9...`).

```go
    return hmac.Equal([]byte(signature), []byte(expectedMAC))
```

`hmac.Equal` does a **constant-time comparison**. This is critical for security: a regular `==` comparison short-circuits (returns false as soon as one byte differs), which leaks timing information. An attacker could figure out the correct signature one byte at a time. `hmac.Equal` always takes the same amount of time regardless of where the mismatch is.

---

## Key Go Patterns Summary

1. **Error handling**: `value, err := fn()` then `if err != nil { handle }`
2. **Goroutines**: `go fn()` launches concurrent work, `sync.WaitGroup` tracks them, `chan struct{}` signals them to stop
3. **Mutex**: `Lock()` / `Unlock()` around shared data, never hold during I/O
4. **`select`**: wait on multiple channels, like a concurrent switch statement
5. **`defer`**: cleanup that runs when the function returns
6. **Struct tags**: metadata on fields (`` `json:"name"` ``) for serialization
7. **Multiple return values**: `(result, error)` is the standard pattern
8. **Pointers**: `*Type` means reference, `&value` takes the address
9. **Slices**: dynamic arrays with `append`, `copy`, `[1:]` slicing
10. **Channels**: goroutine communication, `close()` for broadcast signals
