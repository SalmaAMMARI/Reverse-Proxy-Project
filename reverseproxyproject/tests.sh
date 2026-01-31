echo ""
echo "11. TESTING STICKY SESSIONS"
echo "Making 5 requests with same session..."
session_backends=()
for i in {1..5}; do
    # Use curl with cookie jar
    response=$(curl -s -c cookies.txt -b cookies.txt http://localhost:8080/)
    echo "Request $i: $response"
    session_backends+=("$response")
done

# Check if all requests went to same backend
if [ "${session_backends[0]}" = "${session_backends[1]}" ] && \
   [ "${session_backends[1]}" = "${session_backends[2]}" ]; then
    echo "STICKY SESSIONS: WORKING"
else
    echo "STICKY SESSIONS: NOT WORKING"
fi

echo ""
echo "12. TESTING WEIGHTED LOAD BALANCING"
echo "Making 30 requests to see weight distribution..."

declare -A weighted_hits
weighted_hits["8082"]=0
weighted_hits["8083"]=0
weighted_hits["8084"]=0

for i in {1..30}; do
    response=$(curl -s http://localhost:8080/)
    
    if [ "$response" = "BACKEND_8082" ]; then
        weighted_hits["8082"]=$((weighted_hits["8082"] + 1))
    elif [ "$response" = "BACKEND_8083" ]; then
        weighted_hits["8083"]=$((weighted_hits["8083"] + 1))
    elif [ "$response" = "BACKEND_8084" ]; then
        weighted_hits["8084"]=$((weighted_hits["8084"] + 1))
    fi
    
    sleep 0.1
done

echo ""
echo "WEIGHTED LOAD BALANCING RESULTS (expected ratio 3:2:1):"
echo "Backend 8082 (weight 3): ${weighted_hits["8082"]} hits"
echo "Backend 8083 (weight 2): ${weighted_hits["8083"]} hits"
echo "Backend 8084 (weight 1): ${weighted_hits["8084"]} hits"

# Calculate ratios
total_weighted=$((weighted_hits["8082"] + weighted_hits["8083"] + weighted_hits["8084"]))
if [ $total_weighted -gt 0 ]; then
    ratio_8082=$((weighted_hits["8082"] * 100 / total_weighted))
    ratio_8083=$((weighted_hits["8083"] * 100 / total_weighted))
    ratio_8084=$((weighted_hits["8084"] * 100 / total_weighted))
    echo "Percentages: 8082: ${ratio_8082}%, 8083: ${ratio_8083}%, 8084: ${ratio_8084}%"
fi

echo ""
echo "13. TESTING ADMIN API ENDPOINTS"
echo "GET /status:"
curl -s http://localhost:8081/status | jq '. | {total_backends, alive_backends, strategy}' 2>/dev/null || curl -s http://localhost:8081/status | head -50

echo ""
echo "GET /config:"
curl -s http://localhost:8081/config | jq '.' 2>/dev/null || curl -s http://localhost:8081/config | head -50

if curl -s http://localhost:8081/sessions 2>/dev/null | grep -q "sticky sessions"; then
    echo ""
    echo "GET /sessions (if enabled):"
    curl -s http://localhost:8081/sessions | jq '.' 2>/dev/null || curl -s http://localhost:8081/sessions | head -30
fi