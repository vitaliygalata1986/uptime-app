#!/usr/bin/env python3
"""Проверка живого API; секреты остаются в закрытом временном каталоге."""

import argparse
import http.cookiejar
import json
import os
from pathlib import Path
import secrets
import tempfile
import urllib.error
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", default="http://localhost:8080")
    parser.add_argument("--origin", default="http://localhost:3000")
    parser.add_argument("--state-dir", type=Path)
    parser.add_argument("--phase", choices=("full", "prepare", "verify"), default="full")
    args = parser.parse_args()
    if args.phase != "full" and args.state_dir is None:
        parser.error("--state-dir is required for restart checks")

    if args.phase == "full":
        with tempfile.TemporaryDirectory(prefix="uptime-auth-smoke-") as directory:
            run(args, Path(directory))
    else:
        run(args, args.state_dir)


def run(args, directory):
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    cookie_file = directory / "cookies.txt"
    state_file = directory / "state.json"
    jar = http.cookiejar.MozillaCookieJar(str(cookie_file))
    if args.phase == "verify":
        jar.load(ignore_discard=True, ignore_expires=True)
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))

    def request(method, path, expected, payload=None, access=None):
        headers = {"Origin": args.origin}
        if access:
            headers["Authorization"] = "Bearer " + access
        data = None
        if payload is not None:
            data = json.dumps(payload).encode()
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(args.url.rstrip("/") + path, data=data, headers=headers, method=method)
        try:
            response = client.open(req, timeout=10)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            status, body = response.status, response.read()
        jar.save(ignore_discard=True, ignore_expires=True)
        os.chmod(cookie_file, 0o600)
        if status != expected:
            raise RuntimeError(f"{method} {path}: expected {expected}, got {status}")
        print(f"{method} {path}: {status}")
        return json.loads(body) if body else None

    request("GET", "/healthz", 200)
    if args.phase == "verify":
        state = json.loads(state_file.read_text())
        me = request("GET", "/api/auth/me", 200, access=state["access_token"])
        assert me == state["user"], "user changed after container recreation"
        refreshed = request("POST", "/api/auth/refresh", 200, {})
        assert refreshed["user"] == state["user"], "session lost after container recreation"
        request("POST", "/api/auth/logout", 204, {})
        request("POST", "/api/auth/refresh", 401, {})
        cookie_file.unlink()
        state_file.unlink()
        print("Persistence after container recreation: OK")
        return

    credentials = {"email": "smoke-" + secrets.token_hex(8) + "@example.com", "password": secrets.token_urlsafe(32)}
    registered = request("POST", "/api/auth/register", 201, credentials)
    assert set(registered) == {"user", "access_token", "token_type", "expires_in"}
    assert registered["token_type"] == "Bearer" and registered["expires_in"] == 900
    me = request("GET", "/api/auth/me", 200, access=registered["access_token"])
    assert me == registered["user"]
    refreshed = request("POST", "/api/auth/refresh", 200, {})
    assert refreshed["user"] == me
    request("POST", "/api/auth/logout", 204, {})
    assert not list(jar), "logout did not remove refresh cookie"
    request("POST", "/api/auth/logout", 204, {})
    request("POST", "/api/auth/refresh", 401, {})
    request("GET", "/api/auth/me", 200, access=refreshed["access_token"])
    print("Registration → me → refresh → logout → refresh denied: OK")

    if args.phase == "prepare":
        logged = request("POST", "/api/auth/login", 200, credentials)
        fd = os.open(state_file, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as output:
            json.dump({"user": logged["user"], "access_token": logged["access_token"]}, output)
        print("Cookie file and user saved for restart verification; token values are not printed")


if __name__ == "__main__":
    main()
