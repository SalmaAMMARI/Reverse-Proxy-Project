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
