#!/usr/bin/env python3
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import struct
import subprocess
import sys
import time
from urllib.parse import urlparse
from urllib.request import urlopen

NOTION_URL = "https://" + "app." + "notion.com/"


def find_edge():
    roots = [os.environ.get("ProgramFiles(x86)"), os.environ.get("ProgramFiles"), os.environ.get("LOCALAPPDATA")]
    for root in roots:
        if not root:
            continue
        candidate = Path(root) / "Microsoft" / "Edge" / "Application" / "msedge.exe"
        if candidate.is_file():
            return candidate
    raise RuntimeError("Microsoft Edge not found")


def get_json(url, timeout=3):
    with urlopen(url, timeout=timeout) as response:
        return json.loads(response.read().decode("utf-8"))


def wait_targets(port, timeout=60):
    deadline = time.time() + timeout
    last_error = None
    while time.time() < deadline:
        try:
            return get_json(f"http://127.0.0.1:{port}/json/list")
        except Exception as exc:
            last_error = exc
            time.sleep(0.5)
    raise RuntimeError(f"Edge DevTools endpoint did not start: {last_error}")


def recv_exact(sock, count):
    chunks = []
    remaining = count
    while remaining:
        chunk = sock.recv(remaining)
        if not chunk:
            raise RuntimeError("WebSocket closed unexpectedly")
        chunks.append(chunk)
        remaining -= len(chunk)
    return b"".join(chunks)


def send_frame(sock, payload, opcode=1):
    payload = payload if isinstance(payload, bytes) else payload.encode("utf-8")
    mask = secrets.token_bytes(4)
    length = len(payload)
    header = bytearray([0x80 | opcode])
    if length < 126:
        header.append(0x80 | length)
    elif length < 65536:
        header.append(0x80 | 126)
        header.extend(struct.pack("!H", length))
    else:
        header.append(0x80 | 127)
        header.extend(struct.pack("!Q", length))
    header.extend(mask)
    masked = bytes(value ^ mask[index % 4] for index, value in enumerate(payload))
    sock.sendall(bytes(header) + masked)


def recv_frame(sock):
    first, second = recv_exact(sock, 2)
    opcode = first & 0x0F
    length = second & 0x7F
    masked = bool(second & 0x80)
    if length == 126:
        length = struct.unpack("!H", recv_exact(sock, 2))[0]
    elif length == 127:
        length = struct.unpack("!Q", recv_exact(sock, 8))[0]
    mask = recv_exact(sock, 4) if masked else None
    payload = recv_exact(sock, length)
    if mask:
        payload = bytes(value ^ mask[index % 4] for index, value in enumerate(payload))
    return opcode, payload


def cdp_call(websocket_url, method, params):
    parsed = urlparse(websocket_url)
    if parsed.scheme != "ws" or parsed.hostname not in {"127.0.0.1", "localhost"}:
        raise RuntimeError("Refusing non-local DevTools WebSocket")
    port = parsed.port or 80
    sock = socket.create_connection((parsed.hostname, port), timeout=10)
    try:
        key = base64.b64encode(secrets.token_bytes(16)).decode("ascii")
        request = (
            f"GET {parsed.path or '/'} HTTP/1.1\r\n"
            f"Host: {parsed.hostname}:{port}\r\n"
            "Upgrade: websocket\r\n"
            "Connection: Upgrade\r\n"
            f"Sec-WebSocket-Key: {key}\r\n"
            "Sec-WebSocket-Version: 13\r\n\r\n"
        )
        sock.sendall(request.encode("ascii"))
        response = b""
        while b"\r\n\r\n" not in response:
            response += sock.recv(4096)
            if len(response) > 65536:
                raise RuntimeError("Oversized WebSocket handshake")
        if b" 101 " not in response.split(b"\r\n", 1)[0]:
            raise RuntimeError("Edge rejected WebSocket connection")
        message = json.dumps({"id": 1, "method": method, "params": params}, separators=(",", ":"))
        send_frame(sock, message)
        fragments = bytearray()
        while True:
            opcode, payload = recv_frame(sock)
            if opcode == 8:
                raise RuntimeError("DevTools WebSocket closed before response")
            if opcode == 9:
                send_frame(sock, payload, opcode=10)
                continue
            if opcode in (1, 0):
                fragments.extend(payload)
                try:
                    decoded = json.loads(fragments.decode("utf-8"))
                except (UnicodeDecodeError, json.JSONDecodeError):
                    continue
                fragments.clear()
                if decoded.get("id") == 1:
                    if "error" in decoded:
                        raise RuntimeError("CDP error: " + str(decoded["error"].get("message", "unknown")))
                    return decoded.get("result", {})
    finally:
        sock.close()


def main():
    parser = argparse.ArgumentParser(description="Open a dedicated Edge profile, obtain Notion cookies locally, and start the gateway")
    parser.add_argument("--project-root", default=os.getcwd())
    parser.add_argument("--space-id", default="")
    parser.add_argument("--debug-port", type=int, default=9223)
    args = parser.parse_args()

    project = Path(args.project_root).resolve()
    if not (project / "go.mod").is_file():
        raise RuntimeError(f"go.mod not found in {project}")
    if not shutil.which("go"):
        raise RuntimeError("Go is not available in PATH")

    space_id = args.space_id.strip() or input("Вставь spaceId из Payload getAvailableModels: ").strip()
    if not space_id:
        raise RuntimeError("spaceId is required")

    edge = find_edge()
    local_app_data = Path(os.environ.get("LOCALAPPDATA", project))
    profile = local_app_data / "NotionCodexGateway" / "BrowserProfile"
    profile.mkdir(parents=True, exist_ok=True)

    command = [
        str(edge),
        f"--remote-debugging-port={args.debug_port}",
        f"--user-data-dir={profile}",
        "--no-first-run",
        "--no-default-browser-check",
        "--new-window",
        NOTION_URL,
    ]
    print("Открываю отдельный Edge-профиль...")
    subprocess.Popen(command, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    wait_targets(args.debug_port)
    input("Войди в Notion в открывшемся Edge. Когда страница загрузится, нажми Enter здесь: ")

    targets = get_json(f"http://127.0.0.1:{args.debug_port}/json/list", timeout=5)
    page = next((target for target in targets if target.get("type") == "page" and target.get("url", "").startswith(NOTION_URL)), None)
    if not page:
        raise RuntimeError("Open app.notion.com in the dedicated Edge window")

    result = cdp_call(page["webSocketDebuggerUrl"], "Network.getCookies", {"urls": [NOTION_URL]})
    cookies = [cookie for cookie in result.get("cookies", []) if cookie.get("domain", "").lstrip(".").endswith(("notion.com", "notion.so"))]
    if not cookies:
        raise RuntimeError("No Notion cookies found; confirm login completed")
    cookie_header = "; ".join(f"{cookie['name']}={cookie['value']}" for cookie in cookies)

    env = os.environ.copy()
    env.update({
        "NOTION_CODEX_LIVE": "true",
        "NOTION_CODEX_MOCK": "false",
        "NOTION_CODEX_DEBUG": "true",
        "NOTION_CODEX_TRACE_CONTENT": "true",
        "NOTION_SPACE_ID": space_id,
        "NOTION_TIMEZONE": "Europe/Simferopol",
        "NOTION_SESSION_COOKIE": cookie_header,
    })
    print(f"Cookies получены локально: {len(cookies)} шт. Значения не записаны в файл.")
    print("Расширенное логирование включено: NOTION_CODEX_DEBUG=true")
    print("Трассировка content patch-ов включена: NOTION_CODEX_TRACE_CONTENT=true")
    print("Запускаю gateway. Остановить: Ctrl+C")
    try:
        return subprocess.run(["go", "run", "./cmd/gateway"], cwd=project, env=env, check=False).returncode
    finally:
        cookie_header = ""
        env.pop("NOTION_SESSION_COOKIE", None)


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        print("\nОстановлено.")
        raise SystemExit(130)
    except Exception as exc:
        print(f"Ошибка: {exc}", file=sys.stderr)
        raise SystemExit(1)
