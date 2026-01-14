# test.ps1

Write-Host "=== REVERSE PROXY TEST ===" -ForegroundColor Green

#  Kill old processes
Write-Host "1. Killing old processes..."
taskkill /f /im python.exe 2>$null
taskkill /f /im reverse-proxy.exe 2>$null
Start-Sleep -Seconds 2

#  Create test files
Write-Host "2. Creating test files..."
"this is the server of port 8082" > test82.txt
"this is the server of port 8083" > test83.txt
"this is the server of port 8084" > test84.txt

# Start backends
Write-Host "3. Starting 3 backends..."
Start-Process python -ArgumentList "-m http.server 8082" -WindowStyle Hidden
Start-Process python -ArgumentList "-m http.server 8083" -WindowStyle Hidden
Start-Process python -ArgumentList "-m http.server 8084" -WindowStyle Hidden
Start-Sleep -Seconds 3

#  Test backends directly
Write-Host "4. Testing backends directly..."
Write-Host "   8082:" $(curl.exe -s http://localhost:8082/test82.txt)
Write-Host "   8083:" $(curl.exe -s http://localhost:8083/test83.txt)
Write-Host "   8084:" $(curl.exe -s http://localhost:8084/test84.txt)

#  Update config
Write-Host "5. Updating config..."
@'
{
  "backends": [
    "http://localhost:8082",
    "http://localhost:8083",
    "http://localhost:8084"
  ]
}
'@ > config.json

#  Build and start proxy
Write-Host "6. Building proxy..."
go build -o reverse-proxy.exe main.go
Write-Host "   Starting proxy..."
Start-Process .\reverse-proxy.exe -WindowStyle Hidden
Start-Sleep -Seconds 5

# Test Admin API
Write-Host "7. Testing Admin API..."
Write-Host "   /health:" $(curl.exe -s http://localhost:8081/health)

# Test Proxy
Write-Host "8. Testing Proxy (6 requests)..."
for ($i=1; $i -le 6; $i++) {
    $resp = curl.exe -s http://localhost:8080/test82.txt
    Write-Host "   Request $i : $resp"
    Start-Sleep -Milliseconds 300
}

#  Cleanup
Write-Host "9. Cleaning up..."
taskkill /f /im python.exe 2>$null
taskkill /f /im reverse-proxy.exe 2>$null
Remove-Item test82.txt, test83.txt, test84.txt 2>$null

Write-Host "`n=== DONE ==="