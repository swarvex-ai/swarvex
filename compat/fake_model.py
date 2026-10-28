"""Fake model server for agent tests (E-32): answers the Anthropic Messages API (Claude Code) and the
OpenAI Responses API (Codex) with one fixed text and logs every request (path, auth headers, model)
to stdout. Used to check the runner's sandbox and key proxy end to end without a real model.
Only for tests, never in the lab stack or in production."""
import json, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

TEXT = "SWARVEX-AGENT-OK"


class Modell(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def sse(self, ereignisse):
        self.send_response(200)
        self.send_header("content-type", "text/event-stream")
        self.send_header("cache-control", "no-cache")
        self.send_header("connection", "close")
        self.end_headers()
        for art, daten in ereignisse:
            self.wfile.write(f"event: {art}\ndata: {json.dumps(daten)}\n\n".encode())
        self.wfile.flush()
        self.close_connection = True

    def json(self, daten):
        roh = json.dumps(daten).encode()
        self.send_response(200)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(roh)))
        self.end_headers()
        self.wfile.write(roh)

    def do_GET(self):
        self.protokoll({})
        if self.path.split("?")[0] == "/v1/models" and self.headers.get("chatgpt-account-id"):
            return self.json({"models": []})  # Codex-Backend (ChatGPT-Abo): eigenes Format
        self.json({"object": "list", "data": [{"id": "fake", "object": "model"}]})

    def do_POST(self):
        anfrage = json.loads(self.rfile.read(int(self.headers.get("content-length", 0))) or b"{}")
        self.protokoll(anfrage)
        pfad = self.path.split("?")[0]
        if pfad == "/v1/messages/count_tokens":
            return self.json({"input_tokens": 10})
        if pfad == "/v1/messages":
            nachricht = {"id": "msg_1", "type": "message", "role": "assistant", "model": anfrage.get("model"),
                         "content": [], "stop_reason": None, "usage": {"input_tokens": 100, "output_tokens": 1}}
            if not anfrage.get("stream"):
                nachricht.update(content=[{"type": "text", "text": TEXT}], stop_reason="end_turn",
                                 usage={"input_tokens": 100, "output_tokens": 5})
                return self.json(nachricht)
            return self.sse([
                ("message_start", {"type": "message_start", "message": nachricht}),
                ("content_block_start", {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}),
                ("content_block_delta", {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": TEXT}}),
                ("content_block_stop", {"type": "content_block_stop", "index": 0}),
                ("message_delta", {"type": "message_delta", "delta": {"stop_reason": "end_turn"}, "usage": {"output_tokens": 5}}),
                ("message_stop", {"type": "message_stop"}),
            ])
        if pfad in ("/v1/responses", "/responses"):
            item = {"id": "msg_1", "type": "message", "role": "assistant", "status": "completed",
                    "content": [{"type": "output_text", "text": TEXT, "annotations": []}]}
            antwort = {"id": "resp_1", "object": "response", "status": "completed", "model": anfrage.get("model"),
                       "output": [item], "usage": {"input_tokens": 100, "output_tokens": 5, "total_tokens": 105,
                                                   "input_tokens_details": {"cached_tokens": 0},
                                                   "output_tokens_details": {"reasoning_tokens": 0}}}
            return self.sse([
                ("response.created", {"type": "response.created", "response": {**antwort, "status": "in_progress", "output": [], "usage": None}}),
                ("response.output_item.added", {"type": "response.output_item.added", "output_index": 0,
                                                "item": {**item, "status": "in_progress", "content": []}}),
                ("response.output_text.delta", {"type": "response.output_text.delta", "item_id": "msg_1", "output_index": 0, "content_index": 0, "delta": TEXT}),
                ("response.output_item.done", {"type": "response.output_item.done", "output_index": 0, "item": item}),
                ("response.completed", {"type": "response.completed", "response": antwort}),
            ])
        self.send_response(404)
        self.send_header("content-length", "0")
        self.end_headers()

    def protokoll(self, anfrage):
        print(json.dumps({"path": self.path, "authorization": self.headers.get("authorization"),
                          "x-api-key": self.headers.get("x-api-key"), "anthropic-beta": self.headers.get("anthropic-beta"),
                          "chatgpt-account-id": self.headers.get("chatgpt-account-id"),
                          "model": anfrage.get("model"), "stream": anfrage.get("stream")}), flush=True)

    def log_message(self, *a):
        pass


ThreadingHTTPServer(("0.0.0.0", int(sys.argv[1])), Modell).serve_forever()
