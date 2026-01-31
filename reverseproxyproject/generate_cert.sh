#!/bin/bash
# Generate self-signed SSL certificates for testing

echo "Generating self-signed SSL certificates..."
echo ""

# Generate private key
openssl genrsa -out key.pem 2048

# Generate certificate signing request
openssl req -new -key key.pem -out csr.pem -subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"

# Generate self-signed certificate
openssl x509 -req -days 365 -in csr.pem -signkey key.pem -out cert.pem

# Clean up CSR
rm csr.pem

echo ""
echo "Certificates generated:"
echo "  - cert.pem: SSL certificate"
echo "  - key.pem: Private key"
echo ""
echo "To enable HTTPS, set in config.json:"
echo '  "enable_https": true,'
echo '  "cert_file": "cert.pem",'
echo '  "key_file": "key.pem"'