Let's treat this like a "how to think in Go" guide, not just a bag of tips.
I'll walk through the core ideas and back them with concrete code.

---

## 1. What "idiomatic Go" actually means

Patterns you'll see over and over in good Go code:

* **Simple, explicit control flow** over clever abstraction.
* **Small interfaces, concrete types.**
* **Errors as values**, handled locally.
* **Composition over inheritance.**
* **Tool-enforced style** (`gofmt`, `go vet`, linters).
* **Concurrency with goroutines + channels + context**, not just thread pools.

The official "Effective Go" doc and "Go Code Review Comments" are still the baseline for style and idioms.([go.dev][1])
Google's and Uber's Go style guides build on the same ideas, adding real-world patterns from large codebases.([google.github.io][2])

---

## 2. Tooling & workflow

### Formatting & imports

* **Always run `gofmt` (or `goimports`)** on save.
* IDEs (GoLand, VS Code Go extension, etc.) can do this automatically.
* `goimports` = `gofmt` + auto-fix imports. Recommended.

```bash
go install golang.org/x/tools/cmd/goimports@latest
```

Then configure your editor to use `goimports` instead of `gofmt`.

### Static checks

Use at least:

* `go vet ./...`
* A linter bundle like `golangci-lint` (includes many checks).([cockroachlabs.atlassian.net][3])

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run ./...
```

### Modules

Modern Go means **modules**, not GOPATH.

```bash
mkdir myapp && cd myapp
go mod init example.com/myapp
```

`go.mod` tracks your dependencies and Go version.([go.dev][4])

---

## 3. Project layout

Don't over-engineer it. Use the official layout guidance & only add complexity as needed.([go.dev][5])

### Small/medium service

```txt
myapp/
 go.mod
 cmd/myapp/
  main.go
 internal/
  http/
   server.go
  service/
   user.go
  storage/
   postgres.go
```

* `cmd/myapp`: entrypoint, **tiny `main`** that wires things together.
* `internal`: implementation details you don't want imported by other modules (enforced by the compiler).([go.dev][5])

`cmd/myapp/main.go`:

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "example.com/myapp/internal/http"
    "example.com/myapp/internal/service"
    "example.com/myapp/internal/storage"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(logger)

    ctx := context.Background()

    db, err := storage.OpenPostgres(ctx, os.Getenv("DATABASE_URL"))
    if err != nil {
        logger.Error("open database", "err", err)
        os.Exit(1)
    }
    defer db.Close()

    userSvc := service.NewUserService(db, logger)
    srv := http.NewServer(userSvc, logger)

    if err := srv.Run(ctx, ":8080"); err != nil {
        logger.Error("server exited", "err", err)
        os.Exit(1)
    }
}
```

Notice:

* `main` is just wiring.
* Real work is in `internal/*` packages.
* `context` and `slog` are used from the top.

---

## 4. Naming & basic style

Idiomatic Go naming rules (from Code Review Comments + Google style guide):([go.dev][6])

* **Packages**: short, lower-case, no underscores (`http`, `json`, `auth`, `user`).
* **Exported names**: `CamelCase`, meaningful (`UserService`, `CreateUser`).
* **Unexported names**: `lowerCamel`, often short: `svc`, `cfg`, `id`.
* Prefer **short, context-heavy names** over long "Javaish" names.

Bad:

```go
package userService

type User_service_struct struct {
    UserName string
}
```

Better:

```go
package user // package name usually a noun, not "service"

type Service struct {
    logger *slog.Logger
}
```

Local vars in small scope can be very short:

```go
for i, u := range users {
    // i, u are fine here
}
```

---

## 5. Zero values, construction & options

Go types have meaningful **zero values**; use them.

Bad: always requiring constructors for simple values.

```go
type Config struct {
    Timeout time.Duration
}

func NewConfig() Config {
    return Config{
        Timeout: 5 * time.Second,
    }
}
```

Better:

```go
type Config struct {
    Timeout time.Duration
}

// Zero-value (0) means "use default".
func (c *Config) timeout() time.Duration {
    if c.Timeout == 0 {
        return 5 * time.Second
    }
    return c.Timeout
}
```

### Option functions (for more complex cases)

For non-trivial config, use functional options sparingly.

```go
type Server struct {
    addr string
    timeout time.Duration
    logger *slog.Logger
}

type ServerOption func(*Server)

func WithTimeout(d time.Duration) ServerOption {
    return func(s *Server) { s.timeout = d }
}

func WithLogger(l *slog.Logger) ServerOption {
    return func(s *Server) { s.logger = l }
}

func NewServer(addr string, opts ...ServerOption) *Server {
    s := &Server{
        addr: addr,
        timeout: 5 * time.Second,
        logger: slog.Default(),
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

---

## 6. Value vs pointer receivers

Rule of thumb (rough but practical):

Use **pointer receivers** when:

* The method mutates the receiver.
* The type is large or contains `sync.Mutex`, `sync.Map`, etc.

Use **value receivers** when:

* The type is small (a few fields) and logically immutable.
* You want copying to be cheap.

Example:

```go
type User struct {
    ID int64
    Name string
}

func (u User) DisplayName() string { // value is fine: small & immutable
    if u.Name == "" {
        return "anonymous"
    }
    return u.Name
}

type Counter struct {
    mu sync.Mutex
    n int
}

func (c *Counter) Inc() { // pointer: mutation + mutex
    c.mu.Lock()
    defer c.mu.Unlock()
    c.n++
}

func (c *Counter) Value() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.n
}
```

---

## 7. Interfaces: "accept interfaces, return structs"

Community-standard guideline: define small interfaces at the **consumer**, return concrete types from constructors.([Medium][7])

Bad (God interface, defined by provider):

```go
type Storage interface {
    GetUser(ctx context.Context, id int64) (User, error)
    SaveUser(ctx context.Context, u User) error
    DeleteUser(ctx context.Context, id int64) error
    // ... more methods
}

type PostgresStorage struct { /* ... */ }

func NewStorage(dsn string) Storage { // returns interface
    // ...
}
```

Issues:

* Hard to extend `PostgresStorage` without breaking interface.
* Hard to use concrete methods in same package.
* Tests have to reimplement the full interface.

Better:

```go
// internal/storage/postgres.go
type PostgresStorage struct {
    db *sql.DB
}

func NewPostgresStorage(db *sql.DB) *PostgresStorage {
    return &PostgresStorage{db: db}
}

func (s *PostgresStorage) GetUser(ctx context.Context, id int64) (User, error) {
    // ...
}

// internal/service/user.go

// UserStore is defined where it's used, with only what service needs.
type UserStore interface {
    GetUser(ctx context.Context, id int64) (User, error)
}

type Service struct {
    store UserStore
    logger *slog.Logger
}

func NewService(store UserStore, logger *slog.Logger) *Service {
    return &Service{store: store, logger: logger}
}
```

* `Service` depends on `UserStore` interface it defines.
* `PostgresStorage` is concrete and can be used directly elsewhere.

**Keep interfaces tiny**, often 1–3 methods.

---

## 8. Error handling

Errors are **values**, not exceptions. You'll see `if err != nil` everywhere — that's normal & idiomatic.([Wikipedia][8])

### Basic pattern

```go
u, err := repo.GetUser(ctx, id)
if err != nil {
    return nil, fmt.Errorf("get user %d: %w", id, err)
}
```

Notes:

* Wrap with context using `%w` so callers can use `errors.Is`/`errors.As`.([go.dev][1])
* Avoid custom logging at *every* level; typically log at boundaries (HTTP handler, CLI, background worker loop).

### Sentinel & typed errors

```go
var ErrUserNotFound = errors.New("user not found")

func (s *PostgresStorage) GetUser(ctx context.Context, id int64) (User, error) {
    // ...
    if errors.Is(err, sql.ErrNoRows) {
        return User{}, ErrUserNotFound
    }
    return User{}, fmt.Errorf("select user: %w", err)
}

func (s *Service) GetUserProfile(ctx context.Context, id int64) (Profile, error) {
    u, err := s.store.GetUser(ctx, id)
    if err != nil {
        if errors.Is(err, storage.ErrUserNotFound) {
            return Profile{}, ErrProfileNotFound
        }
        return Profile{}, fmt.Errorf("get user profile: %w", err)
    }
    // ...
}
```

### When to `panic`

* Library code: basically never; use errors.
* `main`: `panic` only for truly unrecoverable programmer bugs; for runtime issues prefer graceful exit with `os.Exit(1)` after logging.

---

## 9. Context: cancellation & timeouts

`context.Context` is how you control lifetime & cancellation of work. Official guidance: pass it as the first parameter after the receiver and propagate it down.([go.dev][9])

Signature shape:

```go
func (s *Service) GetUser(ctx context.Context, id int64) (User, error) {
    // always pass ctx down
    return s.store.GetUser(ctx, id)
}
```

For operations that can block (HTTP, DB, RPC, expensive CPU work), add timeouts higher up:

```go
ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
defer cancel()

u, err := s.userSvc.GetUser(ctx, id)
```

Do **not** stash `context.Context` in structs; treat it as per-request / per-operation data.

---

## 10. Concurrency: goroutines, channels, errgroup

Go's concurrency mantra is "don't communicate by sharing memory; share memory by communicating."([madappgang.com][10])

### Lightweight goroutines

```go
go func() {
    if err := worker.Run(ctx); err != nil {
        logger.Error("worker failed", "err", err)
    }
}()
```

### Worker pool with channels (classic pattern)

```go
func ProcessJobs(ctx context.Context, jobs <-chan Job, workers int) error {
    var g errgroup.Group
    g.SetLimit(workers) // prevent runaway goroutines

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case job, ok := <-jobs:
            if !ok {
                return g.Wait()
            }

            job := job // capture loop variable
            g.Go(func() error {
                return processJob(ctx, job)
            })
        }
    }
}
```

Here we use `golang.org/x/sync/errgroup` to tie goroutines together and propagate errors & cancellation.([Go Packages][11])

### When to use channels vs just goroutines+sync

* Use **channels** when you're modelling a pipeline, fan-in/fan-out, or backpressure.
* Use **errgroup + context** when you just want to run N related tasks concurrently and fail fast on first error.
* Use **mutexes** for shared state that doesn't fit clean channel patterns; don't force channels everywhere.

### Race detector

Turn it on often:

```bash
go test -race ./...
```

It'll catch data races in concurrent code.([Uber][12])

---

## 11. Generics (Go 1.18+): modern but restrained

Generics are mature now (Go 1.18+ and improved in later versions). Use them **for reusable helpers & collections**, not to turn everything into templates.([go.dev][9])

Also keep in mind: naive generics can hurt performance in some cases due to how they're implemented.([planetscale.com][13])

### Simple generic helper

```go
// Map applies fn to each element of in and returns the new slice.
func Map[T any, R any](in []T, fn func(T) R) []R {
    out := make([]R, len(in))
    for i, v := range in {
        out[i] = fn(v)
    }
    return out
}

names := []string{"alice", "bob"}
lengths := Map(names, func(s string) int { return len(s) })
```

### Type constraints

```go
type Number interface {
    ~int | ~int64 | ~float64
}

func Sum[T Number](vals ...T) T {
    var sum T
    for _, v := range vals {
        sum += v
    }
    return sum
}

total := Sum(1, 2, 3)
```

Notes:

* `~int` means "any type whose underlying type is `int`".
* Keep constraints small & meaningful.

### Where generics shine

* Generic collections (sets, maps with helpers).
* Utility functions (`Map`, `Filter`, `Clone`, `Keys`, `Values`).
* Type-safe wrappers around `any`-heavy code (e.g. decoding JSON into `T`).

Where they **don't**:

* Business logic that's already clear with concrete types.
* Huge generic hierarchies that mimic C++ STL style.

---

## 12. Logging: modern structured logging with `slog`

Go 1.21 added `log/slog`, a structured, leveled logging API in the stdlib.([go.dev][14])

Basic usage:

```go
package main

import "log/slog"

func main() {
    slog.Info("user logged in", "user_id", 123, "ip", "1.2.3.4")
}
```

Custom JSON logger:

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
slog.SetDefault(logger)

slog.Info("order created",
    "order_id", 42,
    "user_id", 123,
)
```

Guidelines:

* Log **structured data** as key/value pairs: "user_id", id.
* Log at **boundaries** (HTTP handler, worker main loop) instead of everywhere.
* Pass `*slog.Logger` into your services rather than using globals (except maybe the default logger in `main`).

---

## 13. Testing: table-driven & fuzzing

Idiomatic tests are **table-driven** and live next to the code: `foo.go` / `foo_test.go`.([go.dev][6])

Example:

```go
func NormalizeEmail(s string) string {
    return strings.ToLower(strings.TrimSpace(s))
}

func TestNormalizeEmail(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name string
        in string
        want string
    }{
        {"simple", "USER@EXAMPLE.COM", "user@example.com"},
        {"with spaces", " user@example.com ", "user@example.com"},
        {"already normalized", "user@example.com", "user@example.com"},
    }

    for _, tt := range tests {
        tt := tt // capture
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            got := NormalizeEmail(tt.in)
            if got != tt.want {
                t.Fatalf("NormalizeEmail(%q) = %q, want %q", tt.in, got, tt.want)
            }
        })
    }
}
```

Go 1.18+ also supports **fuzzing** (`testing.F`), which is great for parsing and serialization; you can add:

```go
func FuzzNormalizeEmail(f *testing.F) {
    f.Add("USER@EXAMPLE.COM")
    f.Fuzz(func(t *testing.T, s string) {
        _ = NormalizeEmail(s) // ensure it doesn't panic
    })
}
```

---

## 14. A concrete refactor: "from not-so-Go to idiomatic Go"

### Before

```go
// user_service.go
package userservice

import (
    "fmt"
    "log"
    "time"
)

type UserService struct {
    Repository UserRepository
}

type UserRepository interface {
    FindUserById(id int) (User, error)
}

func (us *UserService) GetUserName(id int) (string, error) {
    log.Printf("Getting user with id %d", id)

    user, err := us.Repository.FindUserById(id)
    if err != nil {
        return "", err
    }

    if user.RegistrationDate.After(time.Now()) {
        return "", fmt.Errorf("registration date is invalid")
    }

    return user.Name, nil
}
```

Problems:

* No `context`.
* Logs with unstructured `log.Printf`.
* Interface is defined by provider, not consumer.
* No error wrapping.
* Uses `int` instead of something clearer (e.g., `int64`).

### After

```go
// internal/service/user.go
package service

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "time"
)

type User struct {
    ID int64
    Name string
    RegistrationDate time.Time
}

var ErrInvalidRegistrationDate = errors.New("invalid registration date")

// UserStore is tiny, consumer-defined interface.
type UserStore interface {
    GetUser(ctx context.Context, id int64) (User, error)
}

type UserService struct {
    store UserStore
    logger *slog.Logger
}

func NewUserService(store UserStore, logger *slog.Logger) *UserService {
    return &UserService{store: store, logger: logger}
}

func (s *UserService) GetUserName(ctx context.Context, id int64) (string, error) {
    s.logger.Info("get user", "user_id", id)

    u, err := s.store.GetUser(ctx, id)
    if err != nil {
        return "", fmt.Errorf("get user %d: %w", id, err)
    }

    if u.RegistrationDate.After(time.Now()) {
        return "", fmt.Errorf("user %d: %w", id, ErrInvalidRegistrationDate)
    }

    return u.Name, nil
}
```

We've made it:

* `context`-aware.
* Structured-log friendly.
* Error-wrapped and traceable.
* Interface small, at consumer.

---

## 15. Mental checklist for "is this idiomatic?"

When you review Go code, ask:

* Did `gofmt` / `goimports` run? Are imports sorted?
* Are names short but clear? Package names lower-case?
* Is `context.Context` used for cancelling work that might block?
* Are errors wrapped with context and checked with `errors.Is`/`errors.As`?
* Are interfaces small and defined at the consumer?
* Are goroutines tied to a context or `errgroup`, so they can't leak?
* Is `slog` (or another structured logger) used instead of ad-hoc prints?
* Is generics usage focused on reusable helpers, not everything?

If the answers are mostly "yes", you're firmly in modern, idiomatic territory.

---

If you want, next step I can take a chunk of your existing Go code and do a line-by-line "idiomatic makeover" on it.

[1]: https://go.dev/doc/effective_go
[2]: https://google.github.io/styleguide/go/
[3]: https://cockroachlabs.atlassian.net/wiki/spaces/CRDB/pages/181371303/Go%2BGolang%2Bcoding%2Bguidelines
[4]: https://go.dev/ref/mod
[5]: https://go.dev/doc/modules/layout
[6]: https://go.dev/wiki/CodeReviewComments
[7]: https://medium.com/@vishnuganb/idiomatic-go-return-structs-accept-interfaces-and-write-cleaner-code-31155c4fea01
[8]: https://en.wikipedia.org/wiki/Go_(programming_language)
[9]: https://go.dev/doc/
[10]: https://madappgang.com/blog/go-best-practices-inspired-by-go-proverbs/
[11]: https://pkg.go.dev/golang.org/x/sync/errgroup
[12]: https://www.uber.com/en-US/blog/data-race-patterns-in-go/
[13]: https://planetscale.com/blog/generics-can-make-your-go-code-slower
[14]: https://go.dev/blog/slog
