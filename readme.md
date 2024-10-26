# 🏃‍♂️ Go Concurrency Patterns Lab

A hands-on exploration of advanced concurrency patterns and techniques from
"Concurrency in Go" by Katherine Cox-Buday.
This repo focuses on implementing and understanding scalable concurrency patterns beyond the basic primitives.

## 🎯 What's This All About?

This is my laboratory for mastering Go's advanced concurrency patterns.
Each implementation includes detailed examples of how these patterns can be used to solve
real-world concurrent programming challenges.

## 🔍 Important Note

These patterns focus on managing concurrent operations **within a single process**.
They are NOT distributed systems patterns. If you're looking for distributed patterns (like distributed locks, consensus, or cross-node communication),
check out resources on distributed systems with Go instead.

## 🧪 Patterns Explored

### Core Patterns
- **Tee**: Split values from a channel into multiple destinations
- **Bridge**: Consume values from a sequence of channels
- **Fan-Out/Fan-In**: Distribute work and collect results
- **Or-Channel**: Combine multiple done channels into a single done channel
- **Pipeline**: Chain together multiple processing steps
- **Error Groups**: Handle errors across multiple goroutines (TODO)

### Advanced Techniques
- **Heartbeat**: Monitor goroutine health in production systems (TODO)
- **Replicated Requests**: Manage redundant requests for reliability (TODO)
- **Rate Limiting**: Control resource consumption (TODO)
- **Self-Healing**: Implement goroutines that recover from failures (TODO)
- **Pooling**: Manage groups of worker goroutines (TODO)
- **Queuing**: Handle backpressure and work distribution (TODO)

## 📚 Inspired By

These implementations are based on patterns from "Concurrency in Go" by Katherine Cox-Buday,
with my own variations and experiments added in.
