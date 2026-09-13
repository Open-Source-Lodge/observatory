# A stand-in for the Anthropic API for the demo GIF. It answers a draft
# request with the same draft, and a check with the same verdicts. The script
# assets/demo.sh starts this file.
import http.server
import json
import time

DRAFT = {
    "title": "No print statements",
    "rule": "Use the logger, not print; do not call print in the code.",
    "why": "The logger has levels and a format; the output of print is lost in production.",
}

VERDICTS = {"results": [
    {"id": "OBS-001", "pass": False, "files": ["client.py"],
     "reason": "client.py line 1 imports requests and line 4 calls requests.get; the rule asks for httpx."},
    {"id": "OBS-002", "pass": True, "files": [],
     "reason": "The changes have no documentation and no comments."},
    {"id": "OBS-003", "pass": True, "files": [],
     "reason": "The changes have no print statement."},
]}


class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        prompt = self.rfile.read(int(self.headers["Content-Length"])).decode()
        answer = DRAFT if "You write a rule" in prompt else VERDICTS
        time.sleep(2)  # so that the GIF shows the progress spinner
        body = json.dumps({
            "content": [{"type": "text", "text": json.dumps(answer)}],
            "stop_reason": "end_turn",
            "usage": {"input_tokens": 1240, "output_tokens": 96},
        }).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


http.server.HTTPServer(("127.0.0.1", 8089), Handler).serve_forever()
