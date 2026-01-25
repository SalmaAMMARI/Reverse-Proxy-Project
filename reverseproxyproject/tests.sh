#!/bin/bash

echo "LOAD BALANCING AND FORWARDING TEST"
echo "Time: $(date)"
echo ""

cleanup() {
    echo "Cleaning up..."
    kill $PROXY_PID 2>/dev/null
    pkill -f "python.*server" 2>/dev/null
    rm -f test*.txt proxy.log server*.py 2>/dev/null
}

trap cleanup EXIT INT TERM

echo "1. Cleaning existing processes..."
pkill -f "python.*server" 2>/dev/null
pkill -f "reverse-proxy" 2>/dev/null
sleep 2

echo "2. Creating backend servers that identify themselves..."

cat > server8082.py << 'EOF'
from http.server import HTTPServer, BaseHTTPRequestHandler

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-type', 'text/plain')
        self.end_headers()
        self.wfile.write(b"BACKEND_8082")
    
    def log_message(self, *args):
        pass

HTTPServer(('', 8082), Handler).serve_forever()
EOF

cat > server8083.py << 'EOF'
from http.server import HTTPServer, BaseHTTPRequestHandler

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-type', 'text/plain')
        self.end_headers()
        self.wfile.write(b"BACKEND_8083")
    
    def log_message(self, *args):
        pass

HTTPServer(('', 8083), Handler).serve_forever()
EOF

cat > server8084.py << 'EOF'
from http.server import HTTPServer, BaseHTTPRequestHandler

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-type', 'text/plain')
        self.end_headers()
        self.wfile.write(b"BACKEND_8084")
    
    def log_message(self, *args):
        pass

HTTPServer(('', 8084), Handler).serve_forever()
EOF

echo "3. Starting backend servers..."
python3 server8082.py > /dev/null 2>&1 &
BACKEND1_PID=$!
python3 server8083.py > /dev/null 2>&1 &
BACKEND2_PID=$!
python3 server8084.py > /dev/null 2>&1 &
BACKEND3_PID=$!
echo "Backends started on ports: 8082, 8083, 8084"
sleep 2

echo "4. Testing backends directly..."
echo "Port 8082: $(curl -s http://localhost:8082/)"
echo "Port 8083: $(curl -s http://localhost:8083/)"
echo "Port 8084: $(curl -s http://localhost:8084/)"

echo "5. Configuring proxy..."
cd /workspaces/Reverse-Proxy-Project/reverseproxyproject

cat > config.json << 'EOF'
{
  "port": 8080,
  "strategy": "round-robin",
  "backends": [
    "http://localhost:8082",
    "http://localhost:8083",
    "http://localhost:8084"
  ]
}
EOF

echo "6. Building proxy..."
go build -o reverse-proxy main.go
if [ $? -ne 0 ]; then
    echo "BUILD FAILED"
    exit 1
fi
echo "Build successful"

echo "7. Starting proxy..."
./reverse-proxy > proxy.log 2>&1 &
PROXY_PID=$!
echo "Proxy started (PID: $PROXY_PID)"
sleep 5

echo ""
echo "8. FORWARDING TEST"
echo ""

echo "Testing single request through proxy..."
response=$(curl -s http://localhost:8080/)
echo "Response: $response"
echo ""

echo "9. LOAD BALANCING TEST"
echo "Making 12 requests to see distribution..."

declare -A backend_hits
backend_hits["8082"]=0
backend_hits["8083"]=0
backend_hits["8084"]=0

for i in {1..12}; do
    response=$(curl -s http://localhost:8080/)
    
    if [ "$response" = "BACKEND_8082" ]; then
        backend_hits["8082"]=$((backend_hits["8082"] + 1))
        echo "Request $i: Backend 8082"
    elif [ "$response" = "BACKEND_8083" ]; then
        backend_hits["8083"]=$((backend_hits["8083"] + 1))
        echo "Request $i: Backend 8083"
    elif [ "$response" = "BACKEND_8084" ]; then
        backend_hits["8084"]=$((backend_hits["8084"] + 1))
        echo "Request $i: Backend 8084"
    else
        echo "Request $i: Unknown response: $response"
    fi
    
    sleep 0.3
done

echo ""
echo "10. LOAD BALANCING RESULTS"
echo "Backend 8082 hits: ${backend_hits["8082"]}"
echo "Backend 8083 hits: ${backend_hits["8083"]}"
echo "Backend 8084 hits: ${backend_hits["8084"]}"
echo ""

total_hits=$((backend_hits["8082"] + backend_hits["8083"] + backend_hits["8084"]))
active_backends=0

for backend in 8082 8083 8084; do
    if [ ${backend_hits[$backend]} -gt 0 ]; then
        active_backends=$((active_backends + 1))
    fi
done

echo "Total requests: $total_hits"
echo "Active backends: $active_backends/3"

if [ $active_backends -ge 2 ]; then
    echo "LOAD BALANCING: WORKING (multiple backends used)"
else
    echo "LOAD BALANCING: NOT WORKING (only one backend used)"
fi

echo ""
echo "Proxy running on: http://localhost:8080"
echo "Admin API on: http://localhost:8081"
echo "Press Ctrl+C to stop"

wait $PROXY_PID