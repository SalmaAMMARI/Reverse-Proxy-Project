# **Go Reverse Proxy Server - Complete Implementation**

## ** Project Overview**
A production-ready, concurrent load-balancing reverse proxy server with health monitoring and admin API, implemented in Go. This project consolidates the requirements from the academic PDF project specification with production-ready features from the Reintech article.

## ** Validated Requirements from PDF Project**

### **1. Core Architecture (100% Completed)**
- [x] **Reverse Proxy Core**: Intercepts requests and forwards using Round-Robin strategy
- [x] **Health Checker**: Background service periodically pings backend servers
- [x] **Admin API**: Separate endpoints for dynamic backend management
- [x] **Concurrent Design**: Handles multiple requests simultaneously using goroutines

### **2. Data Models with Nested Structs**
```go
type Backend struct {
    URL          *url.URL `json:"url"`
    Alive        bool     `json:"alive"`
    CurrentConns int64    `json:"current_connections"`
    // Additional fields from PDF: mux sync.RwMutex
}

type ServerPool struct {
    Backends []*Backend `json:"backends"`
    Current  uint64     `json:"current"` // For Round-Robin
}
```

### **3. Interfaces for Load Balancing**
```go
type LoadBalancer interface {
    GetNextValidPeer() *Backend
    AddBackend(backend *Backend)
    SetBackendStatus(uri *url.URL, alive bool)
}
```
*Implemented via `ServerPool` struct with thread-safe operations*

### **4. Thread-Safe Server Pool**
- [x] `sync.RWMutex` for concurrent access protection
- [x] Atomic counters for Round-Robin selection
- [x] Only returns "Alive" backends
- [x] Returns HTTP 503 when no backends available

### **5. Proxy Handler Implementation**
- [x] Uses `net/http/httputil.ReverseProxy`
- [x] Context propagation for request cancellation
- [x] Custom error handling for backend failures
- [x] Request/response modification

### **6. Periodic Health Checking**
- [x] Configurable interval (default: 10 seconds)
- [x] Background goroutine with `time.Ticker`
- [x] HTTP GET requests to backend endpoints
- [x] Automatic status updates and logging
- [x] Marks backends as DOWN on connection errors

### **7. Admin API Endpoints**
- [x] **GET /status**: JSON list of all backends with health/load status
- [x] **POST /add**: Dynamically add new backend URLs
- [x] Separate port (8082) for management interface

### **8. Additional PDF Requirements**
- [x] Graceful shutdown handling
- [x] Request timeouts using context package
- [x] Error handling for connection refused
- [x] Clean package structure and documentation

## ** Production Features (From Reintech Article)**

### **Performance Optimizations**
- **Connection Pooling**: Reuses HTTP connections to backends
- **Rate Limiting**: `golang.org/x/time/rate` integration (100 req/sec default)
- **Timeouts**: Configurable read/write/idle timeouts
- **Header Sanitization**: Security hardening by removing dangerous headers

### **Security Features**
- **X-Forwarded Headers**: Proper proxy header injection
- **Error Isolation**: Backend failures don't crash proxy
- **Input Validation**: JSON validation for admin API

### **Operational Excellence**
- **Graceful Shutdown**: Proper SIGINT/SIGTERM handling
- **Structured Logging**: Comprehensive request/response logging
- **Metrics**: Request duration and status code tracking
- **Configuration**: JSON/YAML config file support

## ** Architecture Overview**

### **Pipeline Schema**

```
┌─────────────────────────────────────────────────────────────┐
│                    CLIENT REQUESTS                          │
│                    (Port 8000)                              │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   REVERSE PROXY SERVER                      │
├─────────────────────────────────────────────────────────────┤
│  ┌────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Rate       │  │ Load         │  │ Connection       │   │
│  │ Limiter    │──► Balancer     │──► Pool &          │   │
│  │ (100 RPS)  │  │ (Round-Robin)│  │ Headers         │   │
│  └────────────┘  └──────────────┘  └──────────────────┘   │
└───────────────────────────┬─────────────────────────────────┘
                            │
                ┌───────────┼───────────┐
                ▼           ▼           ▼
    ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
    │   BACKEND 1     │ │   BACKEND 2     │ │   BACKEND N     │
    │   (:9091)       │ │   (:9092)       │ │   (:909X)       │
    │   ┌─────────┐   │ │   ┌─────────┐   │ │   ┌─────────┐   │
    │   │ /health │   │ │   │ /ping   │   │ │   │ /status │   │
    │   └─────────┘   │ │   └─────────┘   │ │   └─────────┘   │
    └─────────────────┘ └─────────────────┘ └─────────────────┘
                ▲           ▲           ▲
                └───────────┼───────────┘
                            │
┌─────────────────────────────────────────────────────────────┐
│                    HEALTH CHECKER                           │
│              (Periodic - Every 10s)                         │
└─────────────────────────────────────────────────────────────┘
```

### **Concurrent Processing Flow**
```
┌─────────────────────────────────────────────────────────────┐
│                    Main Goroutine                           │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ 1. Start HTTP Proxy (:8000)                          │  │
│  │ 2. Start Admin API (:8082)                           │  │
│  │ 3. Start Health Checker Goroutine                    │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
      ┌───────────────────────┼───────────────────────┐
      │                       │                       │
      ▼                       ▼                       ▼
┌─────────────┐       ┌─────────────┐       ┌─────────────────┐
│ Request     │       │ Admin       │       │ Health          │
│ Handler     │       │ API         │       │ Checker         │
│ Goroutines  │       │ Goroutine   │       │ Goroutine       │
│ (Per        │       │ (Persistent)│       │ (Periodic Timer)│
│ Request)    │       │             │       │                 │
└─────────────┘       └─────────────┘       └─────────────────┘
```

### **Data Flow for Single Request**
```
1. Client Request → :8000
2. Rate Limiter Check
3. Load Balancer selects backend (Round-Robin)
4. Increment connection counter (atomic)
5. httputil.ReverseProxy forwards request
6. Add X-Forwarded headers
7. Backend processes request
8. Response returns through proxy
9. Decrement connection counter
10. Log response metrics
```

## ** Health Monitoring System**

```
┌─────────────────────────────────────────────────────────────┐
│                    HEALTH CHECK CYCLE                       │
│                    (Every 10 seconds)                       │
├─────────────────────────────────────────────────────────────┤
│  For each backend in ServerPool:                            │
│                                                            │
│  1. Create HTTP client with 5s timeout                     │
│  2. Send GET request to backend URL                        │
│  3. Check response status code (2xx/3xx = healthy)         │
│  4. Update backend.Alive status                            │
│  5. Log status changes (UP/DOWN transitions)               │
│                                                            │
│  Concurrent checks via goroutines for all backends         │
└─────────────────────────────────────────────────────────────┘
```

## ** Installation & Usage**

### **Prerequisites**
- Go 1.21+ installed
- Ports 8000, 8082, 9091, 9092 available

### **Quick Start**
```bash
# Clone and setup
git clone <repository>
cd go-reverse-proxy

# Install dependencies
go mod tidy

# Start backend servers (separate terminals)
go run test-backend1.go  # Port 9091
go run test-backend2.go  # Port 9092

# Start reverse proxy
go run main.go
```

### **Testing**
```bash
# Test load balancing
curl http://localhost:8000/

# Check admin status
curl http://localhost:8082/status

# Add new backend
curl -X POST http://localhost:8082/add \
  -H "Content-Type: application/json" \
  -d '{"url":"http://localhost:9093"}'
```

```

## ** Performance Characteristics**

### **Concurrency Model**
- **Goroutines**: Lightweight threads for each request
- **Connection Pooling**: Reuse backend connections
- **Atomic Operations**: Lock-free counters for performance
- **Buffered Channels**: For graceful shutdown signaling



## ** Security Considerations**

### **Implemented Security Features**
1. **Header Sanitization**: Removes dangerous client headers
2. **Rate Limiting**: Prevents DDoS attacks
3. **Input Validation**: JSON schema validation in admin API
4. **Error Isolation**: Backend failures contained
5. **Context Timeouts**: Prevents resource exhaustion



### **Load Tests**
- Concurrent connection handling
- Memory usage under load
- CPU utilization patterns
- Failure mode analysis

## ** Learning Outcomes**

### **Go Concepts Mastered**
1. **Concurrency**: Goroutines, channels, sync primitives
2. **Networking**: HTTP servers, reverse proxies, connection management
3. **Error Handling**: Context cancellation, graceful degradation
4. **Performance**: Atomic operations, connection pooling, rate limiting

### **System Design Patterns**
1. **Reverse Proxy Pattern**: Request routing and load distribution
2. **Health Check Pattern**: Proactive service monitoring
3. **Admin Interface Pattern**: Runtime configuration management
4. **Graceful Shutdown Pattern**: Clean service termination




## ** Future Enhancements**

### **Planned Features**
1. **Sticky Sessions**: Session affinity based on cookies/IP
2. **Weighted Load Balancing**: Backend capacity-based distribution
3. **TLS Termination**: SSL/TLS support with automatic cert rotation



## ** Conclusion**

This project successfully implements all requirements from the academic PDF specification while incorporating production-ready features from industry best practices. The result is a robust, scalable reverse proxy solution suitable for both learning and production deployment.

**Key Achievements:**

-  Comprehensive health monitoring
-  Dynamic admin interface
-  ready security features
-  Good performance characteristics

