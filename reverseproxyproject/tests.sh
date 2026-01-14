#!/bin/bash

echo "=== REVERSE PROXY TEST  ==="

#  Kill old processes
echo "1. Killing old processes..."
pkill -f "python.*http.server" 2>/dev/null
pkill -f "reverse-proxy" 2>/dev/null
sleep 2

#  Create test files
echo "2. Creating test files..."
echo "this is the server of port 8082" > test82.txt
echo "this is the server of port 8083" > test83.txt
echo "this is the server of port 8084" > test84.txt

#  Start 3 Python backend servers
echo "3. Starting 3 Python backend servers..."
python3 -m http.server 8082 > /dev/null 2>&1 &
python3 -m http.server 8083 > /dev/null 2>&1 &
python3 -m http.server 8084 > /dev/null 2>&1 &
echo "   Started backends on ports: 8082, 8083, 8084"
sleep 3

#  Test backends directly
echo "4. Testing backends directly..."
echo "   8082: $(curl -s http://localhost:8082/test82.txt)"
echo "   8083: $(curl -s http://localhost:8083/test83.txt)"
echo "   8084: $(curl -s http://localhost:8084/test84.txt)"

#  Update config
echo "5. Updating config..."
cat > config.json << 'EOF'
{
  "backends": [
    "http://localhost:8082",
    "http://localhost:8083",
    "http://localhost:8084"
  ]
}
EOF

#  Build and start proxy
echo "6. Building proxy..."
go build -o reverse-proxy main.go
echo "   Starting proxy..."
./reverse-proxy > proxy.log 2>&1 &
PROXY_PID=$!
sleep 5

# Test Admin API
echo "7. Testing Admin API..."
echo "   /health: $(curl -s http://localhost:8081/health | head -c 100)"

# Test Proxy
echo "8. Testing Proxy (6 requests)..."
for i in {1..6}; do
    resp=$(curl -s http://localhost:8080/test82.txt)
    echo "   Request $i : $resp"
    sleep 0.3
done

#  Cleanup
echo "9. Cleaning up..."
kill $PROXY_PID 2>/dev/null
pkill -f "python.*http.server" 2>/dev/null
rm -f test82.txt test83.txt test84.txt proxy.log

echo "=== DONE ==="