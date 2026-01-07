---
schema_version: 1
id: d5feypr
status: open
blocked-by: []
created: 2026-01-07T23:42:19Z
type: task
priority: 2
---
# Add tests for signal cancellation of hooks

Add tests that verify hooks receive signals when the command is cancelled.

## Test approach:

1. Create a bash hook that:
   - Writes 'started' to a file immediately
   - Sets up trap for TERM/INT signals
   - Writes 'signal-received' to a file when trapped
   - Sleeps forever

2. Test flow:
   - Start 'wt create' with sigCh parameter
   - Poll for 'hook-started' file (with timeout)
   - Send SIGTERM via sigCh
   - Wait for command to finish (with timeout)
   - Verify 'signal-received' file exists

3. Test must have hard 5s timeout to fail fast if something is broken

## Implementation:

Add RunWithSignal(sigCh chan os.Signal, args ...string) to CLI test helper in testing_test.go

Example test structure:
```go
func Test_Hook_Receives_Signal_On_Cancellation(t *testing.T) {
    testTimeout := 5 * time.Second
    deadline := time.Now().Add(testTimeout)
    
    // ... setup hook with trap ...
    
    // Poll for hook to start
    for !cli.FileExists("hook-started") {
        if time.Now().After(deadline) {
            t.Fatal("timeout waiting for hook to start")
        }
        time.Sleep(10 * time.Millisecond)
    }
    
    sigCh <- syscall.SIGTERM
    
    // Wait with timeout
    select {
    case <-done:
    case <-time.After(time.Until(deadline)):
        t.Fatal("timeout waiting for command to finish")
    }
    
    // Verify signal received
}
```
